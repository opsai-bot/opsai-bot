package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
)

type CheckFunc func(ctx context.Context) error

type Checker struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]CheckFunc),
	}
}

func (c *Checker) Register(name string, check CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

type CheckResult struct {
	Status  Status            `json:"status"`
	Details map[string]string `json:"details,omitempty"`
}

func (c *Checker) Check(ctx context.Context) CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := CheckResult{
		Status:  StatusHealthy,
		Details: make(map[string]string, len(c.checks)),
	}

	for name, check := range c.checks {
		if err := check(ctx); err != nil {
			result.Status = StatusUnhealthy
			result.Details[name] = err.Error()
		} else {
			result.Details[name] = "ok"
		}
	}

	return result
}

func (c *Checker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	}
}

func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result := c.Check(r.Context())
		w.Header().Set("Content-Type", "application/json")
		if result.Status == StatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(result)
	}
}
