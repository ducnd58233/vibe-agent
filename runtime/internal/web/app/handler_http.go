package app

import "net/http"

const (
	msgMethodNotAllowed = "method not allowed"
	msgBadForm          = "bad form"
	msgTemplateError    = "template error"
	msgBadAfter         = "bad after"
)

func writeMethodNotAllowed(w http.ResponseWriter) {
	http.Error(w, msgMethodNotAllowed, http.StatusMethodNotAllowed)
}

func writeBadForm(w http.ResponseWriter) {
	http.Error(w, msgBadForm, http.StatusBadRequest)
}

func writeTemplateError(w http.ResponseWriter) {
	http.Error(w, msgTemplateError, http.StatusInternalServerError)
}

func writeBadAfter(w http.ResponseWriter) {
	http.Error(w, msgBadAfter, http.StatusBadRequest)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeMethodNotAllowed(w)
	return false
}
