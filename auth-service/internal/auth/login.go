package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type UserServiceClient struct {
	BaseURL string
	Client  *http.Client
}

func (c UserServiceClient) GetPasswordHash(username string) (string, error) {
	u, err := url.JoinPath(c.BaseURL, "/internal/users/"+username)
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequest(http.MethodGet, u, nil)
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrInvalidCredentials
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("user-service status: %d", resp.StatusCode)
	}

	var out struct {
		PasswordHash string `json:"password_hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.PasswordHash == "" {
		return "", fmt.Errorf("empty hash from user-service")
	}
	return out.PasswordHash, nil
}

func VerifyPassword(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}
