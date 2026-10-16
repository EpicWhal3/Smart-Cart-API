package http

import (
	"encoding/json"
	"errors"
	"log/slog"

	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/max25/smart-cart-service/internal/usecase"
)

// Handlers — HTTP-хендлеры корзины.
type Handlers struct {
	uc     *usecase.CartUseCase
	logger *slog.Logger
}

// NewHandlers создаёт набор HTTP-хендлеров.
func NewHandlers(uc *usecase.CartUseCase, logger *slog.Logger) *Handlers {
	return &Handlers{uc: uc, logger: logger}
}

// GetCart обрабатывает GET /api/v1/cart.
func (h *Handlers) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	view, err := h.uc.GetCart(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newCartResponse(view))
}

// GetBDUI обрабатывает GET /api/v1/cart/bdui.
func (h *Handlers) GetBDUI(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	screen, err := h.uc.GetBDUI(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, screen)
}

// AddItem обрабатывает POST /api/v1/cart/items.
func (h *Handlers) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	var req AddItemRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if _, err := h.uc.AddItem(r.Context(), userID, req.SKUID, req.Quantity); err != nil {
		h.writeError(w, r, err)
		return
	}

	view, err := h.uc.GetCart(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newMutationResponse(view))
}

// UpdateItemQuantity обрабатывает PATCH /api/v1/cart/items/{sku_id}.
func (h *Handlers) UpdateItemQuantity(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	skuID, ok := parseSKU(w, r)
	if !ok {
		return
	}

	var req UpdateQuantityRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if _, err := h.uc.UpdateItemQuantity(r.Context(), userID, skuID, req.Quantity); err != nil {
		h.writeError(w, r, err)
		return
	}

	view, err := h.uc.GetCart(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newMutationResponse(view))
}

// RemoveItem обрабатывает DELETE /api/v1/cart/items/{sku_id}.
func (h *Handlers) RemoveItem(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	skuID, ok := parseSKU(w, r)
	if !ok {
		return
	}

	if _, err := h.uc.RemoveItem(r.Context(), userID, skuID); err != nil {
		h.writeError(w, r, err)
		return
	}

	view, err := h.uc.GetCart(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newMutationResponse(view))
}

// ToggleSelection обрабатывает POST /api/v1/cart/items/{sku_id}/toggle.
func (h *Handlers) ToggleSelection(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	skuID, ok := parseSKU(w, r)
	if !ok {
		return
	}

	if _, err := h.uc.ToggleSelection(r.Context(), userID, skuID); err != nil {
		h.writeError(w, r, err)
		return
	}

	view, err := h.uc.GetCart(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newMutationResponse(view))
}

// ApplyPromoCode обрабатывает POST /api/v1/cart/promo.
func (h *Handlers) ApplyPromoCode(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	var req ApplyPromoRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if _, err := h.uc.ApplyPromoCode(r.Context(), userID, req.Code); err != nil {
		h.writeError(w, r, err)
		return
	}

	view, err := h.uc.GetCart(r.Context(), userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newMutationResponse(view))
}

// requireUserID извлекает user_id из заголовка X-User-Id.
func (h *Handlers) requireUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.Header.Get("X-User-Id"))
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "заголовок X-User-Id обязателен",
			Code:  "invalid_user_id",
		})
		return 0, false
	}

	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || userID <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "заголовок X-User-Id должен быть положительным числом",
			Code:  "invalid_user_id",
		})
		return 0, false
	}

	return userID, true
}

// writeError маппит доменные ошибки в HTTP-статусы.
func (h *Handlers) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := mapError(err)

	if status >= http.StatusInternalServerError {
		h.logger.ErrorContext(r.Context(), "request failed",
			slog.String("error", err.Error()), slog.Int("status", status))
	}

	writeJSON(w, status, ErrorResponse{Error: err.Error(), Code: code})
}

// mapError возвращает HTTP-статус и код для доменной ошибки.
func mapError(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrCartNotFound):
		return http.StatusNotFound, "cart_not_found"
	case errors.Is(err, domain.ErrItemNotFound):
		return http.StatusNotFound, "item_not_found"
	case errors.Is(err, domain.ErrProductNotFound):
		return http.StatusNotFound, "product_not_found"
	case errors.Is(err, domain.ErrPromoCodeNotFound):
		return http.StatusNotFound, "promo_code_not_found"
	case errors.Is(err, domain.ErrPromoCodeInactive):
		return http.StatusConflict, "promo_code_inactive"
	case errors.Is(err, domain.ErrPromoMinAmountNotReached):
		return http.StatusUnprocessableEntity, "promo_min_amount_not_reached"
	case errors.Is(err, domain.ErrInvalidQuantity):
		return http.StatusBadRequest, "invalid_quantity"
	case errors.Is(err, domain.ErrCartEmpty):
		return http.StatusConflict, "cart_empty"
	default:
		return http.StatusInternalServerError, "internal_error"
	}
}

// parseSKU извлекает и валидирует {sku_id} из пути.
func parseSKU(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "sku_id")
	skuID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || skuID <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "sku_id должен быть положительным числом",
			Code:  "invalid_sku_id",
		})
		return 0, false
	}
	return skuID, true
}

// decodeJSON декодирует тело запроса, возвращая false и 400 при ошибке.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "тело запроса обязательно",
			Code:  "invalid_body",
		})
		return false
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "некорректное тело запроса: " + err.Error(),
			Code:  "invalid_body",
		})
		return false
	}

	return true
}

// writeJSON сериализует ответ в JSON.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
