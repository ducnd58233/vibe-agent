package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

const contentTypeJSON = "application/json"

// ErrorBody is the JSON error envelope for API handlers and panic recovery.
type ErrorBody struct {
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// JSON is the only encoder so content-type and encoding stay in one place.
func JSON[T any](w http.ResponseWriter, status int, v T) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// RespondError writes JSON or plain text depending on the request.
func RespondError(w http.ResponseWriter, r *http.Request, status int, msg string) {
	if wantsJSON(r) {
		JSON(w, status, ErrorBody{Message: msg})
		return
	}
	http.Error(w, msg, status)
}

func wantsJSON(r *http.Request) bool {
	if r.Header.Get("HX-Request") != "" {
		return false
	}
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}
