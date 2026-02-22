package model

import "time"

type Analysis struct {
	ID             string    `json:"id"`
	AlertID        string    `json:"alert_id"`
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	RootCause      string    `json:"root_cause"`
	Severity       Severity  `json:"severity"`
	Confidence     float64   `json:"confidence"`
	Explanation    string    `json:"explanation"`
	K8sContext     string    `json:"k8s_context"`
	PromptTokens   int       `json:"prompt_tokens"`
	ResponseTokens int       `json:"response_tokens"`
	LatencyMs      int64     `json:"latency_ms"`
	CreatedAt      time.Time `json:"created_at"`
}

func NewAnalysis(alertID, provider, model string) Analysis {
	return Analysis{
		ID:        generateID(),
		AlertID:   alertID,
		Provider:  provider,
		Model:     model,
		CreatedAt: time.Now().UTC(),
	}
}

func (a Analysis) WithDiagnosis(rootCause string, severity Severity, confidence float64, explanation string) Analysis {
	a.RootCause = rootCause
	a.Severity = severity
	a.Confidence = confidence
	a.Explanation = explanation
	return a
}

func (a Analysis) WithTokenUsage(prompt, response int, latencyMs int64) Analysis {
	a.PromptTokens = prompt
	a.ResponseTokens = response
	a.LatencyMs = latencyMs
	return a
}

func (a Analysis) IsHighConfidence() bool {
	return a.Confidence >= 0.7
}
