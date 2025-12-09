package memory

import (
	"context"
	"sync"
	"time"

	"shortener/internal/domain"
)

var _ domain.URLRepository = (*URLRepository)(nil)

type URLRepository struct {
	mu   sync.RWMutex
	urls map[string]*domain.URL
}

func New() *URLRepository {
	return &URLRepository{
		urls: make(map[string]*domain.URL),
	}
}

func (r *URLRepository) Migrate(ctx context.Context) error {
	return nil
}

func (r *URLRepository) Create(ctx context.Context, code, originalURL string, expiresAt *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.urls[code]; exists {
		return domain.ErrCodeAlreadyExists
	}

	now := time.Now().UTC()
	r.urls[code] = &domain.URL{
		Code:        code,
		OriginalURL: originalURL,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
		ClickCount:  0,
	}
	return nil
}

func (r *URLRepository) GetByCode(ctx context.Context, code string) (*domain.URL, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.urls[code]
	if !ok {
		return nil, domain.ErrURLNotFound
	}

	if u.ExpiresAt != nil && time.Now().After(*u.ExpiresAt) {
		return nil, domain.ErrURLNotFound
	}

	cp := *u
	return &cp, nil
}
