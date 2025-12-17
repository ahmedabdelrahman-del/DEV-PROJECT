package users

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	DB *pgxpool.Pool
}

var (
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidPassword = errors.New("invalid password")
	ErrAlreadyExists   = errors.New("user already exists")
	ErrNotFound        = errors.New("user not found")
)

func validateUsername(u string) error {
	u = strings.TrimSpace(u)
	if len(u) < 3 || len(u) > 32 {
		return ErrInvalidUsername
	}
	for _, r := range u {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' && r != '-' {
			return ErrInvalidUsername
		}
	}
	return nil
}

func validatePassword(p string) error {
	if len(p) < 8 || len(p) > 128 {
		return ErrInvalidPassword
	}
	return nil
}

func (s Store) CreateUser(ctx context.Context, username, password string) error {
	if err := validateUsername(username); err != nil {
		return err
	}
	if err := validatePassword(password); err != nil {
		return err
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	hash := string(hashBytes)

	_, err = s.DB.Exec(ctx, `
		INSERT INTO users (username, password_hash)
		VALUES ($1, $2)
	`, username, hash)

	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (s Store) GetPasswordHash(ctx context.Context, username string) (string, error) {
	if err := validateUsername(username); err != nil {
		return "", err
	}

	var hash string
	err := s.DB.QueryRow(ctx, `SELECT password_hash FROM users WHERE username = $1`, username).Scan(&hash)
	if err != nil {
		return "", ErrNotFound
	}
	return hash, nil
}
