package receipt

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, same as the session index
)

// DefaultLumiStore returns the path Lumi writes its records to.
// VIGIL_LUMI_STORE overrides it for non-default Lumi installations.
func DefaultLumiStore() string {
	if p := os.Getenv("VIGIL_LUMI_STORE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lumi", "lumi.db")
}

// LumiStore reads run records from a Lumi store file.
//
// Lumi's own store package exposes no run reader — runs are write-only from
// the engine's side — so this is a reader over the same file, per ADR-012's
// "the adapter is a reader over an existing store". The store is opened
// read-only: a witness that can write to the record it reports on is not a
// witness.
type LumiStore struct {
	db   *sql.DB
	path string
}

// OpenLumi opens a Lumi store file read-only.
func OpenLumi(path string) (*LumiStore, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("receipt: no Lumi store at %s (is Lumi installed and has it run?)", path)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("receipt: opening %s: %w", path, err)
	}
	return &LumiStore{db: db, path: path}, nil
}

// Close releases the store.
func (s *LumiStore) Close() error { return s.db.Close() }

const runColumns = `id, at, request, steps, forecast_usd, actual_usd,
	overhead_usd, policy_version, attendance, met, held`

// kitColumns are recorded by newer Lumi versions. A witness reads whatever the
// record holds: an older store simply has no kit provenance, which is a fact
// about that run and not an error to raise at whoever asks for the receipt.
const kitColumns = `kit, kit_version, kit_hash, servers, grants`

// hasKitColumns reports whether this store records kit provenance.
func (s *LumiStore) hasKitColumns() bool {
	rows, err := s.db.Query(`PRAGMA table_info(runs)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == "kit" {
			return true
		}
	}
	return false
}

func (s *LumiStore) columns() string {
	if s.hasKitColumns() {
		return runColumns + ", " + kitColumns
	}
	return runColumns
}

// Run loads one run by id.
func (s *LumiStore) Run(id int64) (Receipt, error) {
	row := s.db.QueryRow(
		`SELECT `+s.columns()+` FROM runs WHERE id = ?`, id)
	r, err := s.scanRun(row)
	if err == sql.ErrNoRows {
		return Receipt{}, fmt.Errorf("receipt: no run %d in %s", id, s.path)
	}
	return r, err
}

// LatestRun loads the most recent run.
func (s *LumiStore) LatestRun() (Receipt, error) {
	row := s.db.QueryRow(
		`SELECT ` + s.columns() + ` FROM runs ORDER BY id DESC LIMIT 1`)
	r, err := s.scanRun(row)
	if err == sql.ErrNoRows {
		return Receipt{}, fmt.Errorf("receipt: no runs recorded in %s", s.path)
	}
	return r, err
}

func (s *LumiStore) scanRun(row *sql.Row) (Receipt, error) {
	var r Receipt
	var at string
	var met, held int
	var err error
	if s.hasKitColumns() {
		var kit, kitVersion, kitHash, servers, grants sql.NullString
		err = row.Scan(&r.RunID, &at, &r.Request, &r.Steps,
			&r.ForecastUSD, &r.ActualUSD, &r.OverheadUSD,
			&r.PolicyVersion, &r.Attendance, &met, &held,
			&kit, &kitVersion, &kitHash, &servers, &grants)
		r.Kit, r.KitVersion, r.KitHash = kit.String, kitVersion.String, kitHash.String
		r.Servers, r.Grants = servers.String, grants.String
	} else {
		err = row.Scan(&r.RunID, &at, &r.Request, &r.Steps,
			&r.ForecastUSD, &r.ActualUSD, &r.OverheadUSD,
			&r.PolicyVersion, &r.Attendance, &met, &held)
	}
	if err != nil {
		return Receipt{}, err
	}
	r.At, _ = time.Parse(time.RFC3339, at)
	r.ContractMet = met == 1
	r.Held = held == 1
	r.Source = s.path
	return r, nil
}
