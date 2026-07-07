package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"coursework/ordering/domain"
)

type Handler struct {
	service *domain.OrderService
	logger  *slog.Logger
}

func NewHandler(service *domain.OrderService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	order, attempts, err := h.service.PlaceOrder(r.Context(), req.SKU, req.Quantity)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidQuantity) {
			writeError(w, http.StatusBadRequest, "quantity must be a positive integer")
			return
		}
		h.logger.ErrorContext(r.Context(), "place order failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, toOrderResponse(order, attempts))
}

func toOrderResponse(order *domain.Order, attempts int) orderResponse {
	return orderResponse{
		ID:                order.ID,
		SKU:               order.SKU,
		Quantity:          order.Quantity,
		Status:            string(order.Status),
		FailureReason:     order.FailureReason,
		CreatedAt:         order.CreatedAt.Format(time.RFC3339),
		InventoryAttempts: attempts,
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
