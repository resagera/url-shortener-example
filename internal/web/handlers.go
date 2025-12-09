package web

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"shortener/internal/domain"
)

type Handler struct {
	svc    domain.URLService
	logger *slog.Logger
}

func NewHandler(svc domain.URLService, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// RegisterRoutes регистрирует маршруты на стандартном ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// /api/v1/shorten — только POST
	mux.HandleFunc("/api/v1/shorten", h.handleShorten)

	// /{short_key} — всё остальное, начинающееся с "/" (корень)
	// Внутри handleResolve мы сами парсим path и делаем 404 при необходимости.
	mux.HandleFunc("/", h.handleResolve)
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	ShortURL string `json:"short_url"`
}

func (h *Handler) handleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// доп. защита: путь должен быть ровно /api/v1/shorten
	if r.URL.Path != "/api/v1/shorten" {
		http.NotFound(w, r)
		return
	}

	var req shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
	defer cancel()

	code, err := h.svc.Shorten(ctx, req.URL, nil)
	if err != nil {
		h.logger.Error("shorten failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	shortURL := scheme + "://" + r.Host + "/" + code

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201 Created
	_ = json.NewEncoder(w).Encode(shortenResponse{ShortURL: shortURL})
}

func (h *Handler) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		// для всех не-GET по корню — 405
		if r.URL.Path == "/" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// если путь не "/", то пусть спокойно отдаётся 404 ниже
	}

	// Ожидаем путь вида "/{short_key}" без дополнительных сегментов
	path := r.URL.Path

	// Корень "/" — невалидный short_key → 404
	if path == "/" {
		http.NotFound(w, r)
		return
	}

	// Обрезаем ведущий "/"
	code := strings.TrimPrefix(path, "/")

	// Не допускаем дополнительных слэшей: "/a/b" → 404
	if code == "" || strings.Contains(code, "/") {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
	defer cancel()

	originalURL, err := h.svc.Resolve(ctx, code)
	if err != nil {
		if errors.Is(err, domain.ErrURLNotFound) {
			http.NotFound(w, r)
			return
		}
		h.logger.Error("resolve failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// 301 Permanent Redirect
	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
}
