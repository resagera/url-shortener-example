package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"shortener/internal/cache"
	"shortener/internal/domain"
	"shortener/internal/logger"
	memoryrepo "shortener/internal/repo/memory"
	sqliterepo "shortener/internal/repo/sqlite"
	shortenersvc "shortener/internal/service/shortener"
)

func doShorten(t *testing.T, client *http.Client, baseURL, urlLink string) (code string) {
	t.Helper()

	reqBody := struct {
		URL string `json:"url"`
	}{
		URL: urlLink,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&reqBody); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/shorten", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("shorten request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("shorten status = %d, want 201", resp.StatusCode)
	}

	// ТУТ МЕНЯЕМ структуру
	var respBody struct {
		ShortURL string `json:"short_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if respBody.ShortURL == "" {
		t.Fatalf("empty short_url in response")
	}

	// достаём ключ из short_url
	u, err := url.Parse(respBody.ShortURL)
	if err != nil {
		t.Fatalf("invalid short_url: %v", err)
	}
	code = strings.TrimPrefix(u.Path, "/")
	if code == "" {
		t.Fatalf("empty code extracted from short_url %q", respBody.ShortURL)
	}

	return code
}

func doResolve(t *testing.T, client *http.Client, baseURL, code string) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, baseURL+"/"+code, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("resolve request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("resolve status = %d, want 301", resp.StatusCode)
	}

	loc, err := resp.Location()
	if err != nil {
		t.Fatalf("get Location: %v", err)
	}
	return loc.String()
}

func TestShortener_Load_BothRepos(t *testing.T) {
	type repoFactory func(t *testing.T) (domain.URLRepository, func())

	tests := []struct {
		name string
		new  repoFactory
	}{
		{
			name: "SQLite",
			new: func(t *testing.T) (domain.URLRepository, func()) {
				t.Helper()
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "shortener_test.db")

				db, err := sqliterepo.Open(dbPath)
				if err != nil {
					t.Fatalf("open db: %v", err)
				}

				repo := sqliterepo.New(db)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := repo.Migrate(ctx); err != nil {
					t.Fatalf("migrate: %v", err)
				}

				cleanup := func() {
					_ = db.Close()
				}

				return repo, cleanup
			},
		},
		{
			name: "InMemory",
			new: func(t *testing.T) (domain.URLRepository, func()) {
				t.Helper()
				repo := memoryrepo.New()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := repo.Migrate(ctx); err != nil {
					t.Fatalf("migrate: %v", err)
				}

				return repo, func() {}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo, cleanup := tc.new(t)
			defer cleanup()

			// сервис и HTTP
			urlCache := cache.NewURLCache(100_000)
			svc := shortenersvc.NewURLService(repo, urlCache, logger.NewNoopLogger())
			h := NewHandler(svc, logger.NewNoopLogger())

			mux := http.NewServeMux()
			h.RegisterRoutes(mux)
			ts := httptest.NewServer(mux)
			defer ts.Close()

			client := &http.Client{
				Timeout: 5 * time.Second,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			const (
				createN = 1000
				readK   = 10
			)

			codes := make([]string, createN)

			// --- создание ссылок ---

			startCreate := time.Now()

			var wgCreate sync.WaitGroup
			for i := 0; i < createN; i++ {
				wgCreate.Add(1)
				i := i
				go func() {
					defer wgCreate.Done()
					url := "https://example.com/resource/" + time.Now().Format(time.RFC3339Nano)
					code := doShorten(t, client, ts.URL, url)
					codes[i] = code
				}()
			}
			wgCreate.Wait()

			elapsedCreate := time.Since(startCreate)
			createPerSec := float64(createN) / elapsedCreate.Seconds()
			t.Logf("[%s] Created %d short URLs in %s (~%.0f create/s)", tc.name, createN, elapsedCreate, createPerSec)

			for i, c := range codes {
				if c == "" {
					t.Fatalf("empty code at index %d", i)
				}
			}

			// --- чтение ссылок ---

			totalReads := createN * readK

			startRead := time.Now()

			var wgRead sync.WaitGroup
			for i := 0; i < createN; i++ {
				code := codes[i]
				for j := 0; j < readK; j++ {
					wgRead.Add(1)
					go func(code string) {
						defer wgRead.Done()
						loc := doResolve(t, client, ts.URL, code)
						if loc == "" {
							t.Fatalf("empty redirect location for code=%s", code)
						}
					}(code)
				}
			}
			wgRead.Wait()

			elapsedRead := time.Since(startRead)
			readPerSec := float64(totalReads) / elapsedRead.Seconds()
			t.Logf("[%s] Resolved %d short URLs in %s (~%.0f read/s)", tc.name, totalReads, elapsedRead, readPerSec)
		})
	}
}
