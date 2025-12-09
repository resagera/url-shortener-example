package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"shortener/internal/domain"
)

var _ domain.URLRepository = (*URLRepository)(nil)

type URLRepository struct {
	db *sql.DB
}

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(0)

	if _, err = db.Exec(`PRAGMA journal_mode = WAL;`); err != nil {
		return nil, err
	}
	if _, err = db.Exec(`PRAGMA synchronous = NORMAL;`); err != nil {
		return nil, err
	}
	if _, err = db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, err
	}
	if _, err = db.Exec(`PRAGMA busy_timeout = 5000;`); err != nil {
		return nil, err
	}

	return db, nil
}

func New(db *sql.DB) *URLRepository {
	return &URLRepository{db: db}
}

func (r *URLRepository) Migrate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    original_url TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%f', 'now')),
    expires_at DATETIME NULL,
    click_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_urls_code ON urls(code);
`)
	return err
}

func (r *URLRepository) Create(ctx context.Context, code, originalURL string, expiresAt *time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO urls(code, original_url, expires_at) VALUES(?,?,?)`,
		code, originalURL, expiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if sqliteIsUniqueViolation(err) {
			return domain.ErrCodeAlreadyExists
		}
	}
	return err
}

func (r *URLRepository) GetByCode(ctx context.Context, code string) (*domain.URL, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT code, original_url, created_at, expires_at, click_count
FROM urls
WHERE code = ?;
`, code)

	var u domain.URL
	var expires sql.NullTime

	if err := row.Scan(&u.Code, &u.OriginalURL, &u.CreatedAt, &expires, &u.ClickCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrURLNotFound
		}
		return nil, err
	}

	if expires.Valid {
		u.ExpiresAt = &expires.Time
	}

	if u.ExpiresAt != nil && time.Now().After(*u.ExpiresAt) {
		return nil, domain.ErrURLNotFound
	}

	return &u, nil
}

func sqliteIsUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
