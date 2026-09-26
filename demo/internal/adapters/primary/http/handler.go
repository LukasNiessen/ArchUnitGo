// Package http is the primary (driving) adapter: it turns HTTP requests into use
// cases. It knows the transport and nothing about where an order is stored.
package http

import (
	"encoding/json"
	"net/http"

	"github.com/LukasNiessen/ArchUnitGoDemo/internal/application"
)

// Handler carries the one use case the transport exposes.
type Handler struct {
	PlaceOrder application.PlaceOrder
}

type placeOrderRequest struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	Amount     int64  `json:"amount"`
}

// Place handles POST /orders.
func (h Handler) Place(w http.ResponseWriter, r *http.Request) {
	var req placeOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order, err := h.PlaceOrder.Handle(req.ID, req.CustomerID, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(order)
}
