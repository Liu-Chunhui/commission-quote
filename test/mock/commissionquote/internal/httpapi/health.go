package httpapi

import "net/http"

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte("ok")); err != nil {
		requestLogger(r.Context()).Error("Unable to write health response", "operation", "write_response")
	}
}
