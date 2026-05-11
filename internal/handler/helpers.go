package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// parseID parses the chi URL param "id" as a positive int64.
func parseID(r *http.Request) (int64, bool) {
	return parseIDStr(chi.URLParam(r, "id"))
}

// parseIDStr parses a raw string as a positive int64.
func parseIDStr(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
