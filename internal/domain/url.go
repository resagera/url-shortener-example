package domain

import (
	"context"
	"errors"
	"time"
)

type URL struct {
	Code        string
	OriginalURL string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
	ClickCount  int64
}

type URLRepository interface {
	Migrate(ctx context.Context) error
	Create(ctx context.Context, code, originalURL string, expiresAt *time.Time) error
	GetByCode(ctx context.Context, code string) (*URL, error)
}

type URLService interface {
	Shorten(ctx context.Context, originalURL string, expiresAt *time.Time) (string, error)
	Resolve(ctx context.Context, code string) (string, error)
}

var (
	ErrCodeAlreadyExists = errors.New("short code already exists")
	ErrURLNotFound       = errors.New("short url not found")
)
