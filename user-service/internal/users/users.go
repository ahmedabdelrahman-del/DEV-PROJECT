package users

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	DB *pgxpool.Pool
}

var (
	ErrInvalidUsername    = errors.New("invalid username")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrAlreadyExists      = errors.New("user already exists")
	ErrNotFound           = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// 23505 = unique_violation
			if pgErr.Code == "23505" {
				return ErrAlreadyExists
			}
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
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return hash, nil
}

// VerifyCredentials checks whether the provided password matches the stored hash.
// It does NOT return the hash, so callers don't need access to password_hash at all.
func (s Store) VerifyCredentials(ctx context.Context, username, password string) error {
	if err := validateUsername(username); err != nil {
		return err
	}
	// Don't enforce complexity here; login should allow any input length within reason.
	password = strings.TrimSpace(password)
	if password == "" || len(password) > 256 {
		return ErrInvalidCredentials
	}

	hash, err := s.GetPasswordHash(ctx, username)
	if err != nil {
		// Treat not found as invalid creds to avoid user enumeration.
		if errors.Is(err, ErrNotFound) {
			return ErrInvalidCredentials
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}
