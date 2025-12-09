package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"shortener/internal/cache"
	"shortener/internal/logger"
	"shortener/internal/repo/memory"
	shortenersvc "shortener/internal/service/shortener"
)

func newTestServer(t *testing.T) (*httptest.Server, *memory.URLRepository) {
	t.Helper()

	repo := memory.New()
	if err := repo.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	urlCache := cache.NewURLCache(100_000)
	svc := shortenersvc.NewURLService(repo, urlCache, logger.NewNoopLogger())
	h := NewHandler(svc, logger.NewNoopLogger())

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	ts := httptest.NewServer(mux)
	return ts, repo
}

func TestShortenAndRedirect(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// для POST не важно, для GET хотим видеть именно 301 от нашего сервера
			return http.ErrUseLastResponse
		},
	}

	// --- 1. Создание сокращённого URL ---

	origURL := "https://very-long-original-url.com/some/path?param=value"

	reqBody := map[string]string{"url": origURL}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		t.Fatalf("encode: %v", err)
	}

	resp, err := client.Post(ts.URL+"/api/v1/shorten", "application/json", &buf)
	if err != nil {
		t.Fatalf("POST /api/v1/shorten error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var respBody struct {
		ShortURL string `json:"short_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if respBody.ShortURL == "" {
		t.Fatalf("short_url is empty")
	}

	// short_url должен быть полным (с доменом и портом сервиса)
	parsed, err := url.Parse(respBody.ShortURL)
	if err != nil {
		t.Fatalf("short_url is not valid URL: %v", err)
	}
	if parsed.Scheme != "http" {
		t.Fatalf("short_url scheme = %s, want http", parsed.Scheme)
	}
	if parsed.Host == "" {
		t.Fatalf("short_url host is empty")
	}

	// ключ берём из path
	key := strings.TrimPrefix(parsed.Path, "/")
	if key == "" {
		t.Fatalf("short key is empty")
	}
	if l := len(key); l < 7 || l > 10 {
		t.Fatalf("short key length = %d, want between 7 and 10", l)
	}

	// --- 2. Redirect Endpoint /{short_key} ---

	getResp, err := client.Get(ts.URL + "/" + key)
	if err != nil {
		t.Fatalf("GET /{key} error: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusMovedPermanently { // 301
		t.Fatalf("GET /{key} status = %d, want 301", getResp.StatusCode)
	}

	loc, err := getResp.Location()
	if err != nil {
		t.Fatalf("Location header missing/invalid: %v", err)
	}
	if loc.String() != origURL {
		t.Fatalf("Location = %s, want %s", loc.String(), origURL)
	}

	// --- 3. 404 Not Found для несуществующего ключа ---

	resp404, err := client.Get(ts.URL + "/nonexistent123456")
	if err != nil {
		t.Fatalf("GET /nonexistent error: %v", err)
	}
	defer resp404.Body.Close()

	if resp404.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /nonexistent status = %d, want 404", resp404.StatusCode)
	}
}
