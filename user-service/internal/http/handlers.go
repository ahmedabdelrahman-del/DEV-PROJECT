package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"user-service/internal/users"
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
	var req createUserReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	err := h.Users.CreateUser(r.Context(), req.Username, req.Password)
	switch err {
	case nil:
		writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
	case users.ErrInvalidUsername, users.ErrInvalidPassword:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case users.ErrAlreadyExists:
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server error"})
	}
}

func (h Handlers) GetInternalUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	hash, err := h.Users.GetPasswordHash(r.Context(), username)
	switch err {
	case nil:
		writeJSON(w, http.StatusOK, map[string]string{"username": username, "password_hash": hash})
	case users.ErrNotFound:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case users.ErrInvalidUsername:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid username"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server error"})
	}
}
