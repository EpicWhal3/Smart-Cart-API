package http

import (
	"context"
	"log/slog"
	"net/http"

	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/max25/smart-cart-service/internal/usecase"
)

// requestIDHeader — заголовок с идентификатором запроса.
const requestIDHeader = "X-Request-Id"

type ctxKey string

const requestIDKey ctxKey = "request_id"

// NewRouter собирает маршруты и middleware.
func NewRouter(uc *usecase.CartUseCase, logger *slog.Logger, timeout time.Duration) http.Handler {
	handlers := NewHandlers(uc, logger)

	r := chi.NewRouter()
	r.Use(RequestID)
	r.Use(Logging(logger))
	r.Use(Recover(logger))
	if timeout > 0 {
		r.Use(Timeout(timeout))
	}

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1/cart", func(r chi.Router) {
		r.Get("/", handlers.GetCart)
		r.Get("/bdui", handlers.GetBDUI)
		r.Post("/items", handlers.AddItem)
		r.Patch("/items/{sku_id}", handlers.UpdateItemQuantity)
		r.Delete("/items/{sku_id}", handlers.RemoveItem)
		r.Post("/items/{sku_id}/toggle", handlers.ToggleSelection)
		r.Post("/promo", handlers.ApplyPromoCode)
	})

	return r
}

// RequestID прокидывает идентификатор запроса в контекст и заголовок ответа.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logging логирует запросы в JSON через slog.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.InfoContext(r.Context(), "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", RequestIDFromContext(r.Context())),
			)
		})
	}
}

// Recover перехватывает панику и отвечает 500 без падения процесса.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.ErrorContext(r.Context(), "panic recovered",
						slog.Any("panic", rec),
						slog.String("path", r.URL.Path),
						slog.String("request_id", RequestIDFromContext(r.Context())),
					)
					writeJSON(w, http.StatusInternalServerError, ErrorResponse{
						Error: "внутренняя ошибка сервера",
						Code:  "internal_error",
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout ограничивает время обработки запроса.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, d, `{"error":"превышено время обработки запроса","code":"timeout"}`)
	}
}

// RequestIDFromContext возвращает идентификатор запроса из контекста.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}
