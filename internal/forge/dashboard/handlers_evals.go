package dashboard

import (
	"net/http"
	"time"
)

type evalRunJSON struct {
	ID             string     `json:"id"`
	Source         string     `json:"source"`
	TargetName     string     `json:"target_name"`
	SuiteName      string     `json:"suite_name"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	TotalScenarios int        `json:"total_scenarios"`
	Passed         int        `json:"passed"`
	Failed         int        `json:"failed"`
	Skipped        int        `json:"skipped"`
	Status         string     `json:"status"`
}

type evalResultJSON struct {
	ID           string     `json:"id"`
	RunID        string     `json:"run_id"`
	ScenarioName string     `json:"scenario_name"`
	Status       string     `json:"status"`
	DurationMs   *int64     `json:"duration_ms"`
	ErrorMessage *string    `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
}

type evalDetailJSON struct {
	Run     evalRunJSON      `json:"run"`
	Results []evalResultJSON `json:"results"`
}

// handleListEvals returns eval runs with optional source filter and limit.
func (s *Server) handleListEvals(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	limit := queryInt(r, "limit", 50)

	runs, err := s.evalStore.ListRuns(source, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list evals: "+err.Error())
		return
	}

	items := make([]evalRunJSON, 0, len(runs))
	for _, run := range runs {
		items = append(items, evalRunJSON{
			ID:             run.ID,
			Source:         run.Source,
			TargetName:     run.TargetName,
			SuiteName:      run.SuiteName,
			StartedAt:      run.StartedAt,
			FinishedAt:     run.FinishedAt,
			TotalScenarios: run.TotalScenarios,
			Passed:         run.Passed,
			Failed:         run.Failed,
			Skipped:        run.Skipped,
			Status:         run.Status,
		})
	}

	writeJSON(w, http.StatusOK, listResponse{Data: items, Total: len(items)})
}

// handleGetEval returns a single eval run with all its results.
func (s *Server) handleGetEval(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	run, err := s.evalStore.GetRun(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get eval: "+err.Error())
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "eval run not found")
		return
	}

	results, err := s.evalStore.GetResultsByRun(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get eval results: "+err.Error())
		return
	}

	resultItems := make([]evalResultJSON, 0, len(results))
	for _, res := range results {
		resultItems = append(resultItems, evalResultJSON{
			ID:           res.ID,
			RunID:        res.RunID,
			ScenarioName: res.ScenarioName,
			Status:       res.Status,
			DurationMs:   res.DurationMs,
			ErrorMessage: res.ErrorMessage,
			CreatedAt:    res.CreatedAt,
		})
	}

	detail := evalDetailJSON{
		Run: evalRunJSON{
			ID:             run.ID,
			Source:         run.Source,
			TargetName:     run.TargetName,
			SuiteName:      run.SuiteName,
			StartedAt:      run.StartedAt,
			FinishedAt:     run.FinishedAt,
			TotalScenarios: run.TotalScenarios,
			Passed:         run.Passed,
			Failed:         run.Failed,
			Skipped:        run.Skipped,
			Status:         run.Status,
		},
		Results: resultItems,
	}

	writeJSON(w, http.StatusOK, detailResponse{Data: detail})
}
