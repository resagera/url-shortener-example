package logger

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

type asyncHandler struct {
	ch   chan logEntry
	wg   sync.WaitGroup
	done chan struct{}
	out  slog.Handler
}

type logEntry struct {
	ctx context.Context
	rec slog.Record
}

func NewAsyncHandler(out slog.Handler, buffer int) slog.Handler {
	h := &asyncHandler{
		ch:   make(chan logEntry, buffer),
		out:  out,
		done: make(chan struct{}),
	}
	h.wg.Add(1)
	go h.worker()
	return h
}

func (h *asyncHandler) worker() {
	defer h.wg.Done()
	for {
		select {
		case e := <-h.ch:
			_ = h.out.Handle(e.ctx, e.rec)
		case <-h.done:
			// Drain remaining logs
			for e := range h.ch {
				_ = h.out.Handle(e.ctx, e.rec)
			}
			return
		}
	}
}

func (h *asyncHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.out.Enabled(ctx, level)
}

func (h *asyncHandler) Handle(ctx context.Context, rec slog.Record) error {
	select {
	case h.ch <- logEntry{ctx: ctx, rec: rec}:
	default:
		// drop or panic — в продакшене лучше логировать дроп
	}
	return nil
}

func (h *asyncHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Создаём новый asyncHandler с обёрнутым WithAttrs
	return &asyncHandler{
		ch:   h.ch,
		wg:   h.wg,
		done: h.done,
		out:  h.out.WithAttrs(attrs),
	}
}

func (h *asyncHandler) WithGroup(name string) slog.Handler {
	return &asyncHandler{
		ch:   h.ch,
		wg:   h.wg,
		done: h.done,
		out:  h.out.WithGroup(name),
	}
}

func (h *asyncHandler) Close() {
	close(h.done)
	h.wg.Wait()
}

func main() {
	asyncH := NewAsyncHandler(slog.NewTextHandler(os.Stdout, nil), 100)
	logger := slog.New(asyncH)

	logger.Info("hello", "user", "alice")
	logger.Error("oops", "err", "disk full")

	if closer, ok := asyncH.(*asyncHandler); ok {
		closer.Close()
	}
}
