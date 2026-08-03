package guardrails

import (
	"fmt"
	"time"

	"github.com/ceasarb/trovery-tools/internal/forge/shared/storage"
)

// ErrBudgetExceeded is returned when the per-request budget is hit.
var ErrBudgetExceeded = fmt.Errorf("per-request budget exceeded")

// ErrMonthlyCapReached is returned when the monthly budget cap is hit.
var ErrMonthlyCapReached = fmt.Errorf("monthly budget cap reached")

// Ledger is the durable record of spend that the monthly cap is enforced
// against. CostStore is the built-in SQLite implementation, which is
// process-local: on ephemeral or per-instance disks (Cloud Run, Fargate, any
// autoscaled container) it resets on cold start and is not shared between
// instances, so the monthly ceiling holds only within one instance's lifetime.
// Deployments that need a real cross-instance ceiling should supply their own
// implementation backed by shared storage.
//
// Implementations must be safe for concurrent use.
type Ledger interface {
	// MonthlySpend returns total spend for the calendar month containing t.
	MonthlySpend(t time.Time) (float64, error)
	// RecordCost adds costUSD to the ledger for the month containing at.
	RecordCost(agentName string, costUSD float64, at time.Time) error
}

// Budget tracks cost limits and enforcement.
type Budget struct {
	PerRequest float64 // USD limit per request (0 = unlimited)
	Monthly    float64 // USD limit per calendar month (0 = unlimited)
	store      Ledger
}

// New creates a Budget with the given limits.
//
// If ledger is nil, monthly tracking is disabled and the monthly cap is not
// enforced — only the in-process per-request limit applies. Pass an untyped nil
// for that case: a nil *CostStore assigned to the Ledger parameter is a non-nil
// interface holding a nil pointer, which panics on first use rather than
// degrading to "tracking disabled".
func New(perRequest, monthly float64, ledger Ledger) *Budget {
	return &Budget{
		PerRequest: perRequest,
		Monthly:    monthly,
		store:      ledger,
	}
}

// CheckRequestBudget returns ErrBudgetExceeded if currentCost exceeds the per-request limit.
func (b *Budget) CheckRequestBudget(currentCost float64) error {
	if b.PerRequest <= 0 {
		return nil
	}
	if currentCost >= b.PerRequest {
		return ErrBudgetExceeded
	}
	return nil
}

// CheckMonthlyBudget returns ErrMonthlyCapReached if cumulative monthly spend
// meets or exceeds the monthly limit. Returns the remaining budget.
func (b *Budget) CheckMonthlyBudget() (remaining float64, err error) {
	if b.Monthly <= 0 {
		return 0, nil
	}
	if b.store == nil {
		return b.Monthly, nil
	}

	spent, err := b.store.MonthlySpend(time.Now())
	if err != nil {
		return 0, fmt.Errorf("query monthly spend: %w", err)
	}

	remaining = b.Monthly - spent
	if remaining <= 0 {
		return 0, ErrMonthlyCapReached
	}
	return remaining, nil
}

// RecordCost adds a cost entry for the current month.
func (b *Budget) RecordCost(agentName string, cost float64) error {
	if b.store == nil || cost <= 0 {
		return nil
	}
	return b.store.RecordCost(agentName, cost, time.Now())
}

// MonthlyRemaining returns the remaining monthly budget, or -1 if unlimited.
func (b *Budget) MonthlyRemaining() float64 {
	if b.Monthly <= 0 {
		return -1
	}
	if b.store == nil {
		return b.Monthly
	}
	spent, err := b.store.MonthlySpend(time.Now())
	if err != nil {
		return b.Monthly
	}
	remaining := b.Monthly - spent
	if remaining < 0 {
		return 0
	}
	return remaining
}

// CostStore persists monthly cost data in SQLite. It is process-local — see
// Ledger for what that means for the monthly cap under multi-instance or
// ephemeral-disk deployments.
type CostStore struct {
	db *storage.DB
}

var _ Ledger = (*CostStore)(nil)

var costMigrations = []storage.Migration{
	{
		Version:     1,
		Description: "create cost_entries table",
		SQL: `CREATE TABLE IF NOT EXISTS cost_entries (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_name TEXT NOT NULL,
			cost_usd   REAL NOT NULL,
			month_key  TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_cost_entries_month ON cost_entries(month_key);`,
	},
}

// NewCostStore opens or creates a cost tracking database at the given path.
func NewCostStore(path string) (*CostStore, error) {
	db, err := storage.Open(path)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(costMigrations); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate cost store: %w", err)
	}
	return &CostStore{db: db}, nil
}

// Close closes the underlying database.
func (cs *CostStore) Close() error {
	return cs.db.Close()
}

// RecordCost inserts a cost entry for the given month.
func (cs *CostStore) RecordCost(agentName string, costUSD float64, at time.Time) error {
	monthKey := monthKeyFor(at)
	_, err := cs.db.Conn().Exec(
		"INSERT INTO cost_entries (agent_name, cost_usd, month_key) VALUES (?, ?, ?)",
		agentName, costUSD, monthKey,
	)
	return err
}

// MonthlySpend returns the total cost for the calendar month containing t.
func (cs *CostStore) MonthlySpend(t time.Time) (float64, error) {
	monthKey := monthKeyFor(t)
	var total float64
	err := cs.db.Conn().QueryRow(
		"SELECT COALESCE(SUM(cost_usd), 0) FROM cost_entries WHERE month_key = ?",
		monthKey,
	).Scan(&total)
	return total, err
}

// monthKeyFor returns "YYYY-MM" for the given time.
func monthKeyFor(t time.Time) string {
	return t.Format("2006-01")
}
