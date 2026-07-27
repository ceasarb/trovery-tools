package delegation

import (
	"context"
	"testing"
)

func TestOnBehalfOf_RoundTrip(t *testing.T) {
	ctx := WithOnBehalfOf(context.Background(), "signed-assertion")
	got, ok := OnBehalfOf(ctx)
	if !ok {
		t.Fatal("expected assertion present")
	}
	if got != "signed-assertion" {
		t.Fatalf("got %q, want %q", got, "signed-assertion")
	}
}

func TestOnBehalfOf_Absent(t *testing.T) {
	if _, ok := OnBehalfOf(context.Background()); ok {
		t.Fatal("expected no assertion on a bare context")
	}
}

func TestWithOnBehalfOf_EmptyIsNoOp(t *testing.T) {
	// An empty assertion must leave the context untouched, so absent behaves
	// identically to the pre-ADR-008 path.
	base := context.Background()
	ctx := WithOnBehalfOf(base, "")
	if _, ok := OnBehalfOf(ctx); ok {
		t.Fatal("empty assertion must not be stored")
	}
}

func TestOnBehalfOf_LastWriteWins(t *testing.T) {
	ctx := WithOnBehalfOf(context.Background(), "first")
	ctx = WithOnBehalfOf(ctx, "second")
	got, _ := OnBehalfOf(ctx)
	if got != "second" {
		t.Fatalf("got %q, want %q", got, "second")
	}
}
