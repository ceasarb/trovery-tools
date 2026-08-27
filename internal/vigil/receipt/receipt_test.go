package receipt

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedStore creates a Lumi-shaped store file with the given runs, matching the
// schema Lumi's store package migrates (lumi/internal/store/store.go).
func seedStore(t *testing.T, runs int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lumi.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const schema = `CREATE TABLE runs (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		at             TEXT    NOT NULL,
		request        TEXT    NOT NULL,
		steps          INTEGER NOT NULL,
		forecast_usd   REAL    NOT NULL,
		actual_usd     REAL    NOT NULL,
		overhead_usd   REAL    NOT NULL,
		policy_version TEXT    NOT NULL,
		attendance     TEXT    NOT NULL,
		met            INTEGER NOT NULL,
		held           INTEGER NOT NULL
	);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= runs; i++ {
		_, err := db.Exec(`INSERT INTO runs
			(at, request, steps, forecast_usd, actual_usd, overhead_usd,
			 policy_version, attendance, met, held)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			time.Date(2026, 8, 27, 10, i, 0, 0, time.UTC).Format(time.RFC3339),
			"summarize the meeting notes", 4, 0.021, 0.0184, 0.0031,
			"policy-v1", "attended", 1, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestLatestRunPicksNewest(t *testing.T) {
	store, err := OpenLumi(seedStore(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	r, err := store.LatestRun()
	if err != nil {
		t.Fatal(err)
	}
	if r.RunID != 3 {
		t.Fatalf("LatestRun = run %d, want 3", r.RunID)
	}
	if !r.ContractMet || r.Held {
		t.Fatalf("flags: met=%v held=%v, want met=true held=false", r.ContractMet, r.Held)
	}
}

func TestRunByID(t *testing.T) {
	store, err := OpenLumi(seedStore(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	r, err := store.Run(2)
	if err != nil {
		t.Fatal(err)
	}
	if r.RunID != 2 {
		t.Fatalf("Run(2) = run %d", r.RunID)
	}
	if _, err := store.Run(99); err == nil {
		t.Fatal("Run(99) should fail for a missing run")
	}
}

func TestOpenMissingStoreFails(t *testing.T) {
	if _, err := OpenLumi(filepath.Join(t.TempDir(), "nope.db")); err == nil {
		t.Fatal("OpenLumi should fail when the store file does not exist")
	}
}

// TestRenderHonestSentence pins the ADR-012 trust claim: the receipt says
// "recorded by the harness, outside the model's reach" and never claims
// independence.
func TestRenderHonestSentence(t *testing.T) {
	store, err := OpenLumi(seedStore(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	r, err := store.LatestRun()
	if err != nil {
		t.Fatal(err)
	}
	out := RenderText(r)

	if !strings.Contains(out, HonestSentence) {
		t.Fatalf("receipt is missing the honest sentence %q:\n%s", HonestSentence, out)
	}
	if strings.Contains(strings.ToLower(out), "independent") {
		t.Fatalf("receipt overclaims with 'independent' (ADR-012):\n%s", out)
	}
	// User-authored text is quoted and attributed, never stated as fact.
	if !strings.Contains(out, `"summarize the meeting notes" (as given by the user)`) {
		t.Fatalf("request is not rendered as quoted user input:\n%s", out)
	}
}

func TestOutcomeVerdicts(t *testing.T) {
	cases := []struct {
		met, held bool
		want      string
	}{
		{true, false, "contract met"},
		{false, true, "stopped — held for a person"},
		{false, false, "ended without meeting the contract"},
	}
	for _, c := range cases {
		got := Receipt{ContractMet: c.met, Held: c.held}.Outcome()
		if got != c.want {
			t.Errorf("Outcome(met=%v held=%v) = %q, want %q", c.met, c.held, got, c.want)
		}
	}
}
