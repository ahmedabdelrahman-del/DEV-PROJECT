package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"user-service/internal/users"

	"github.com/go-chi/chi/v5"
)

type Handlers struct {
	Users users.Store
}

type createUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h Handlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Prevent large bodies from causing memory pressure.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createUserReq
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	// Ensure there isn't trailing garbage after the first JSON object.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)

	err := h.Users.CreateUser(r.Context(), req.Username, req.Password)
	switch err {
	case nil:
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	case users.ErrInvalidUsername, users.ErrInvalidPassword:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case users.ErrAlreadyExists:
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		// Log the actual error for debugging
		if err != nil {
			fmt.Fprintf(os.Stderr, "CreateUser error: %v\n", err)
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server error"})
	}
}

// Ready returns 200 only when dependencies (like the DB) are reachable.
func (h Handlers) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.Users.DB.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type verifyUserReq struct {
	Password string `json:"password"`
}

// VerifyInternalUser verifies credentials without exposing password hashes.
func (h Handlers) VerifyInternalUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req verifyUserReq
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	req.Password = strings.TrimSpace(req.Password)
	if req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password required"})
		return
	}

	err := h.Users.VerifyCredentials(r.Context(), username, req.Password)
	switch err {
	case nil:
		// No content is enough; auth-service just needs success/failure.
		w.WriteHeader(http.StatusNoContent)
	case users.ErrInvalidUsername:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid username"})
	case users.ErrInvalidCredentials:
		// Avoid user enumeration; respond the same for not found or wrong password.
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server error"})
	}
}
