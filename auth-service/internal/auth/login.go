package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type UserServiceClient struct {
	BaseURL string
	Client  *http.Client
}

func (c UserServiceClient) VerifyCredentials(username, password string) error {
	u, err := url.JoinPath(c.BaseURL, "/internal/users/"+url.PathEscape(username)+"/verify")
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]string{"password": password})
	req, _ := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}
	// Treat both "not found" and "unauthorized" the same to avoid enumeration.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
		return ErrInvalidCredentials
	}
	return fmt.Errorf("user-service status: %d", resp.StatusCode)
}

func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}
