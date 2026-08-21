package guardrails

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempCostStore(t *testing.T) (*CostStore, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cost.db")
	store, err := NewCostStore(path)
	if err != nil {
		t.Fatalf("create cost store: %v", err)
	}
	return store, path
}

func TestPerRequestBudgetUnlimited(t *testing.T) {
	b := New(0, 0, nil)
	if err := b.CheckRequestBudget(100.0); err != nil {
		t.Fatalf("unlimited budget should not error: %v", err)
	}
}

func TestPerRequestBudgetUnderLimit(t *testing.T) {
	b := New(1.0, 0, nil)
	if err := b.CheckRequestBudget(0.50); err != nil {
		t.Fatalf("under limit should not error: %v", err)
	}
}

func TestPerRequestBudgetExactBoundary(t *testing.T) {
	b := New(1.0, 0, nil)
	err := b.CheckRequestBudget(1.0)
	if err != ErrBudgetExceeded {
		t.Fatalf("exact boundary should return ErrBudgetExceeded, got: %v", err)
	}
}

func TestPerRequestBudgetOverLimit(t *testing.T) {
	b := New(0.50, 0, nil)
	err := b.CheckRequestBudget(0.75)
	if err != ErrBudgetExceeded {
		t.Fatalf("over limit should return ErrBudgetExceeded, got: %v", err)
	}
}

func TestMonthlyBudgetUnlimited(t *testing.T) {
	b := New(0, 0, nil)
	remaining, err := b.CheckMonthlyBudget()
	if err != nil {
		t.Fatalf("unlimited should not error: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("unlimited should return 0 remaining, got %f", remaining)
	}
}

func TestMonthlyBudgetNoStore(t *testing.T) {
	b := New(0, 10.0, nil)
	remaining, err := b.CheckMonthlyBudget()
	if err != nil {
		t.Fatalf("no store should not error: %v", err)
	}
	if remaining != 10.0 {
		t.Fatalf("no store should return full budget, got %f", remaining)
	}
}

func TestMonthlyBudgetUnderCap(t *testing.T) {
	store, _ := tempCostStore(t)
	defer store.Close()

	// Record $3 of spend
	store.RecordCost("agent-a", 1.5, time.Now())
	store.RecordCost("agent-a", 1.5, time.Now())

	b := New(0, 10.0, store)
	remaining, err := b.CheckMonthlyBudget()
	if err != nil {
		t.Fatalf("under cap should not error: %v", err)
	}
	if remaining != 7.0 {
		t.Fatalf("expected 7.0 remaining, got %f", remaining)
	}
}

func TestMonthlyBudgetExactCap(t *testing.T) {
	store, _ := tempCostStore(t)
	defer store.Close()

	store.RecordCost("agent-a", 10.0, time.Now())

	b := New(0, 10.0, store)
	_, err := b.CheckMonthlyBudget()
	if err != ErrMonthlyCapReached {
		t.Fatalf("exact cap should return ErrMonthlyCapReached, got: %v", err)
	}
}

func TestMonthlyBudgetOverCap(t *testing.T) {
	store, _ := tempCostStore(t)
	defer store.Close()

	store.RecordCost("agent-a", 12.0, time.Now())

	b := New(0, 10.0, store)
	_, err := b.CheckMonthlyBudget()
	if err != ErrMonthlyCapReached {
		t.Fatalf("over cap should return ErrMonthlyCapReached, got: %v", err)
	}
}

func TestMonthlyBudgetCrossMonth(t *testing.T) {
	store, _ := tempCostStore(t)
	defer store.Close()

	// Record cost in a different month
	lastMonth := time.Now().AddDate(0, -1, 0)
	store.RecordCost("agent-a", 100.0, lastMonth)

	// Current month should still have full budget
	b := New(0, 10.0, store)
	remaining, err := b.CheckMonthlyBudget()
	if err != nil {
		t.Fatalf("different month should not count: %v", err)
	}
	if remaining != 10.0 {
		t.Fatalf("expected full budget, got %f", remaining)
	}
}

func TestRecordCostNilStore(t *testing.T) {
	b := New(0, 0, nil)
	if err := b.RecordCost("agent-a", 1.0); err != nil {
		t.Fatalf("nil store should not error: %v", err)
	}
}

func TestRecordCostZeroCost(t *testing.T) {
	store, _ := tempCostStore(t)
	defer store.Close()

	b := New(0, 10.0, store)
	if err := b.RecordCost("agent-a", 0); err != nil {
		t.Fatalf("zero cost should not error: %v", err)
	}

	// Verify nothing was recorded
	spent, _ := store.MonthlySpend(time.Now())
	if spent != 0 {
		t.Fatalf("zero cost should not record, got %f", spent)
	}
}

func TestMonthlyRemainingUnlimited(t *testing.T) {
	b := New(0, 0, nil)
	if r := b.MonthlyRemaining(); r != -1 {
		t.Fatalf("unlimited should return -1, got %f", r)
	}
}

func TestCostStoreCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cost.db")

	store, err := NewCostStore(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	store.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file should exist")
	}
}

func TestCostStoreMultipleAgents(t *testing.T) {
	store, _ := tempCostStore(t)
	defer store.Close()

	now := time.Now()
	store.RecordCost("agent-a", 3.0, now)
	store.RecordCost("agent-b", 5.0, now)

	// Monthly spend is across all agents
	spent, err := store.MonthlySpend(now)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if spent != 8.0 {
		t.Fatalf("expected 8.0 total, got %f", spent)
	}
}
