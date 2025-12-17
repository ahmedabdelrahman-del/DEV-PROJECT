package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Router(h Handlers) http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Post("/users", h.CreateUser)
	r.Get("/internal/users/{username}", h.GetInternalUser)

	return r
}
