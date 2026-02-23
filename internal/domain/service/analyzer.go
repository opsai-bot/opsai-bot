package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jonny/opsai-bot/internal/domain/model"
	"github.com/jonny/opsai-bot/internal/domain/port/outbound"
)

const maxFollowUpIterations = 3

// Analyzer coordinates LLM analysis of alerts using gathered Kubernetes context.
type Analyzer struct {
	llm outbound.LLMProvider
	k8s outbound.K8sExecutor
}

// NewAnalyzer creates a new Analyzer.
func NewAnalyzer(llm outbound.LLMProvider, k8s outbound.K8sExecutor) *Analyzer {
	return &Analyzer{llm: llm, k8s: k8s}
}

// AnalyzeAlert gathers K8s context for the alert, calls the LLM to produce a
// diagnosis, and runs follow-up queries when the LLM indicates it needs more
// information (up to maxFollowUpIterations rounds).
func (a *Analyzer) AnalyzeAlert(ctx context.Context, alert model.Alert) (model.Analysis, []outbound.SuggestedAction, error) {
	k8sCtx, err := a.gatherK8sContext(ctx, alert)
	if err != nil {
		// Non-fatal: continue with whatever context was collected.
		k8sCtx = fmt.Sprintf("partial k8s context (error: %v)", err)
	}

	req := outbound.DiagnosisRequest{
		AlertSummary: fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Title, alert.Description),
		K8sContext:   k8sCtx,
		Environment:  alert.Environment,
	}

	start := time.Now()
	result, err := a.llm.Diagnose(ctx, req)
	if err != nil {
		return model.Analysis{}, nil, fmt.Errorf("LLM diagnosis failed: %w", err)
	}

	// Follow-up loop.
	for i := 0; i < maxFollowUpIterations && result.NeedsMoreInfo && len(result.FollowUpQueries) > 0; i++ {
		additionalCtx, queryErr := a.runFollowUpQueries(ctx, alert, result.FollowUpQueries)
		if queryErr != nil {
			// Append error info and stop iterating.
			req.K8sContext = req.K8sContext + "\n" + fmt.Sprintf("follow-up query error: %v", queryErr)
			break
		}
		req.K8sContext = req.K8sContext + "\n" + additionalCtx
		result, err = a.llm.Diagnose(ctx, req)
		if err != nil {
			return model.Analysis{}, nil, fmt.Errorf("LLM follow-up diagnosis failed: %w", err)
		}
	}

	latencyMs := time.Since(start).Milliseconds()

	modelInfo, _ := a.llm.ModelInfo(ctx)

	analysis := model.NewAnalysis(alert.ID, modelInfo.Provider, modelInfo.Model).
		WithDiagnosis(result.RootCause, model.Severity(result.Severity), result.Confidence, result.Explanation).
		WithTokenUsage(0, 0, latencyMs)

	analysis = analysis.WithK8sContext(k8sCtx)

	return analysis, result.SuggestedActions, nil
}

// HandleConversation continues an existing conversation thread with a new user message.
func (a *Analyzer) HandleConversation(
	ctx context.Context,
	thread model.ConversationThread,
	userMsg string,
	alertID string,
) (outbound.ConversationResponse, error) {
	// Build LLM message history from thread.
	history := make([]outbound.Message, 0, len(thread.Messages))
	for _, m := range thread.Messages {
		history = append(history, outbound.Message{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}

	req := outbound.ConversationRequest{
		ThreadID:    thread.ThreadID,
		UserMessage: userMsg,
		History:     history,
		AlertID:     alertID,
	}

	resp, err := a.llm.Converse(ctx, req)
	if err != nil {
		return outbound.ConversationResponse{}, fmt.Errorf("LLM conversation failed: %w", err)
	}

	return resp, nil
}

// gatherK8sContext collects resource info, pod logs and events for the alert.
func (a *Analyzer) gatherK8sContext(ctx context.Context, alert model.Alert) (string, error) {
	var parts []string

	if alert.Namespace != "" && alert.Resource != "" {
		res, err := a.k8s.GetResource(ctx, outbound.ResourceQuery{
			Namespace:    alert.Namespace,
			ResourceType: "pod",
			Name:         alert.Resource,
		})
		if err == nil {
			parts = append(parts, "=== Resource ===\n"+res.Raw)
		}

		logs, err := a.k8s.GetPodLogs(ctx, alert.Namespace, alert.Resource, "", 100)
		if err == nil {
			parts = append(parts, "=== Pod Logs ===\n"+logs)
		}

		events, err := a.k8s.GetEvents(ctx, alert.Namespace, alert.Resource)
		if err == nil {
			parts = append(parts, "=== Events ===\n"+events)
		}
	}

	if len(parts) == 0 {
		clusterCtx, err := a.k8s.GetClusterContext(ctx)
		if err != nil {
			return "", fmt.Errorf("no K8s context available: %w", err)
		}
		return clusterCtx, nil
	}

	return strings.Join(parts, "\n\n"), nil
}

// runFollowUpQueries executes each follow-up query and returns the aggregated output.
func (a *Analyzer) runFollowUpQueries(ctx context.Context, alert model.Alert, queries []string) (string, error) {
	var parts []string
	for _, q := range queries {
		// Queries are freeform; attempt to fetch resource description matching the query.
		desc, err := a.k8s.DescribeResource(ctx, alert.Namespace, "pod", q)
		if err != nil {
			parts = append(parts, fmt.Sprintf("query %q: error: %v", q, err))
			continue
		}
		parts = append(parts, fmt.Sprintf("query %q:\n%s", q, desc))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no follow-up query results")
	}
	return strings.Join(parts, "\n\n"), nil
}
