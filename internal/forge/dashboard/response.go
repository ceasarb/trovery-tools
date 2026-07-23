package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// listResponse is the standard envelope for list endpoints.
type listResponse struct {
	Data  interface{} `json:"data"`
	Total int         `json:"total"`
}

// detailResponse is the standard envelope for detail endpoints.
type detailResponse struct {
	Data interface{} `json:"data"`
}

// errorResponse is the standard envelope for error responses.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// queryInt reads an integer query parameter with a default fallback.
func queryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}
