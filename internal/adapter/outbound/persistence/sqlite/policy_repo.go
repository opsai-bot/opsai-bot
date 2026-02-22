package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jonny/opsai-bot/internal/domain/model"
)

// PolicyRepo implements outbound.PolicyRepository using SQLite.
type PolicyRepo struct {
	db *sql.DB
}

// NewPolicyRepo creates a new PolicyRepo backed by the given store.
func NewPolicyRepo(store *Store) *PolicyRepo {
	return &PolicyRepo{db: store.DB}
}

// GetByEnvironment fetches the policy for a specific environment.
func (r *PolicyRepo) GetByEnvironment(ctx context.Context, env string) (model.EnvironmentPolicy, error) {
	const q = `SELECT id, environment, mode, max_auto_risk, approvers, namespaces, custom_rules, enabled
		FROM policies WHERE environment = ?`

	row := r.db.QueryRowContext(ctx, q, env)
	p, err := scanPolicy(row)
	if err == sql.ErrNoRows {
		return model.EnvironmentPolicy{}, fmt.Errorf("policy for environment %s not found", env)
	}
	if err != nil {
		return model.EnvironmentPolicy{}, fmt.Errorf("fetching policy: %w", err)
	}
	return p, nil
}

// GetAll returns all stored environment policies.
func (r *PolicyRepo) GetAll(ctx context.Context) ([]model.EnvironmentPolicy, error) {
	const q = `SELECT id, environment, mode, max_auto_risk, approvers, namespaces, custom_rules, enabled
		FROM policies ORDER BY environment ASC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("querying all policies: %w", err)
	}
	defer rows.Close()

	var results []model.EnvironmentPolicy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning policy: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// Upsert inserts or replaces an environment policy.
func (r *PolicyRepo) Upsert(ctx context.Context, p model.EnvironmentPolicy) error {
	approvers, err := json.Marshal(p.Approvers)
	if err != nil {
		return fmt.Errorf("marshaling approvers: %w", err)
	}
	namespaces, err := json.Marshal(p.Namespaces)
	if err != nil {
		return fmt.Errorf("marshaling namespaces: %w", err)
	}
	customRules, err := json.Marshal(p.CustomRules)
	if err != nil {
		return fmt.Errorf("marshaling custom_rules: %w", err)
	}

	const q = `INSERT INTO policies (id, environment, mode, max_auto_risk, approvers, namespaces, custom_rules, enabled)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(environment) DO UPDATE SET
			id=excluded.id,
			mode=excluded.mode,
			max_auto_risk=excluded.max_auto_risk,
			approvers=excluded.approvers,
			namespaces=excluded.namespaces,
			custom_rules=excluded.custom_rules,
			enabled=excluded.enabled`

	_, err = r.db.ExecContext(ctx, q,
		p.ID, p.Environment, string(p.Mode), p.MaxAutoRisk,
		string(approvers), string(namespaces), string(customRules), p.Enabled,
	)
	if err != nil {
		return fmt.Errorf("upserting policy: %w", err)
	}
	return nil
}

// --- helpers ---

type policyScanner interface {
	Scan(dest ...any) error
}

func scanPolicy(s policyScanner) (model.EnvironmentPolicy, error) {
	var p model.EnvironmentPolicy
	var mode, approversJSON, namespacesJSON, customRulesJSON string

	err := s.Scan(
		&p.ID, &p.Environment, &mode, &p.MaxAutoRisk,
		&approversJSON, &namespacesJSON, &customRulesJSON, &p.Enabled,
	)
	if err != nil {
		return model.EnvironmentPolicy{}, err
	}

	p.Mode = model.PolicyMode(mode)

	if err := json.Unmarshal([]byte(approversJSON), &p.Approvers); err != nil {
		p.Approvers = []string{}
	}
	if err := json.Unmarshal([]byte(namespacesJSON), &p.Namespaces); err != nil {
		p.Namespaces = []string{}
	}
	if err := json.Unmarshal([]byte(customRulesJSON), &p.CustomRules); err != nil {
		p.CustomRules = []model.PolicyRule{}
	}
	return p, nil
}
