package controllers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/antonkarounis/stoic/internal/domain/ports"
)

// httpError writes an appropriate HTTP error response based on the domain error type.
func httpError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ports.ErrNotFound):
		http.Error(w, "Not Found", http.StatusNotFound)
	case errors.Is(err, ports.ErrForbidden):
		http.Error(w, "Forbidden", http.StatusForbidden)
	default:
		slog.Error("internal server error", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
