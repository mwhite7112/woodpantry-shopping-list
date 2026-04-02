package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mwhite7112/woodpantry-shopping-list/internal/logging"
	"github.com/mwhite7112/woodpantry-shopping-list/internal/service"
)

// NewRouter wires the HTTP routes for the shopping-list service.
func NewRouter(_ *service.Service) http.Handler {
	router := chi.NewRouter()
	router.Use(logging.Middleware)
	router.Use(middleware.Recoverer)

	router.Get("/healthz", handleHealth)

	return router
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("ok")) //nolint:errcheck
}
