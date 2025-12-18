package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"auth-service/internal/auth"
)

type Handlers struct {
	UserClient auth.UserServiceClient
	JWTSecret  string
	JWTTTL     time.Duration
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (h Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if err := h.UserClient.VerifyCredentials(req.Username, req.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := auth.SignJWT(h.JWTSecret, req.Username, h.JWTTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func MustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(key + " is required")
	}
	return v
}
