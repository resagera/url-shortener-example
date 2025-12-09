package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"shortener/internal/cache"
	"shortener/internal/domain"
)

type urlService struct {
	repo   domain.URLRepository
	cache  *cache.URLCache
	logger *slog.Logger
}

func NewURLService(repo domain.URLRepository, cache *cache.URLCache, logger *slog.Logger) domain.URLService {
	return &urlService{repo: repo, cache: cache, logger: logger}
}

func (s *urlService) Shorten(ctx context.Context, originalURL string, expiresAt *time.Time) (string, error) {
	const (
		codeLen     = 8
		maxAttempts = 5
	)

	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		code := generateCode(codeLen)

		err := s.repo.Create(ctx, code, originalURL, expiresAt)
		if err == nil {
			s.cache.Set(code, originalURL)
			s.logger.Info("short url created", "code", code, "originalURL", originalURL)
			return code, nil
		}

		if errors.Is(err, domain.ErrCodeAlreadyExists) {
			// Коллизия при многопоточности — генерируем новый код
			lastErr = err
			continue
		}

		// другая ошибка — выходим
		s.logger.Error("failed to create short url: %v", "err", err)
		return "", err
	}

	return "", fmt.Errorf("failed to generate unique short code after %d attempts: %w", maxAttempts, lastErr)
}

func (s *urlService) Resolve(ctx context.Context, code string) (string, error) {
	if url, ok := s.cache.Get(code); ok {
		s.logger.Debug("cache hit: code", "code", code)
		// статистику можно считать асинхронно
		go func() {
			//отправим, например в сервис статистики или в очередь, чтобы потом батчами записывать в кликхаус
		}()
		return url, nil
	}

	s.logger.Debug("cache miss: code", "code", code)

	u, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		if errors.Is(err, domain.ErrURLNotFound) {
			return "", domain.ErrURLNotFound
		}
		return "", err
	}

	s.cache.Set(code, u.OriginalURL)
	go func() {
		//отправим, например в сервис статистики или в очередь, чтобы потом батчами записывать в кликхаус
	}()

	return u.OriginalURL, nil
}

var _ domain.URLService = (*urlService)(nil)
