package app

import "net/http"

const (
	msgMethodNotAllowed = "method not allowed"
	msgBadForm          = "bad form"
	msgTemplateError    = "template error"
	msgBadAfter         = "bad after"
)

func writeMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeHTMXOrError(w, r, http.StatusMethodNotAllowed, msgMethodNotAllowed)
}

func writeBadForm(w http.ResponseWriter, r *http.Request) {
	writeHTMXOrError(w, r, http.StatusBadRequest, msgBadForm)
}

func writeTemplateError(w http.ResponseWriter, r *http.Request) {
	writeHTMXOrError(w, r, http.StatusInternalServerError, msgTemplateError)
}

func writeBadAfter(w http.ResponseWriter, r *http.Request) {
	writeHTMXOrError(w, r, http.StatusBadRequest, msgBadAfter)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeMethodNotAllowed(w, r)
	return false
}
