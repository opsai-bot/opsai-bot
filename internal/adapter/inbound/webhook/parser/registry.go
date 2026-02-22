package parser

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/jonny/opsai-bot/internal/domain/port/inbound"
)

// Registry manages WebhookParser instances and resolves the correct parser per request.
type Registry struct {
	mu      sync.RWMutex
	parsers []inbound.WebhookParser
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a parser to the registry. Parsers are tried in registration order.
func (r *Registry) Register(p inbound.WebhookParser) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsers = append(r.parsers, p)
}

// Resolve returns the first parser that can handle the given request.
func (r *Registry) Resolve(req *http.Request) (inbound.WebhookParser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.parsers {
		if p.CanParse(req) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no parser found for request")
}

// Sources returns the source names of all registered parsers.
func (r *Registry) Sources() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sources := make([]string, len(r.parsers))
	for i, p := range r.parsers {
		sources[i] = p.Source()
	}
	return sources
}
