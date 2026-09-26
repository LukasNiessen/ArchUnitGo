// Package memory is a secondary (driven) adapter: the in-memory implementation
// of the order repository, wired up in cmd/server so the demo runs without a
// database.
package memory

import (
	"errors"
	"sync"

	"github.com/LukasNiessen/ArchUnitGoDemo/internal/application"
	"github.com/LukasNiessen/ArchUnitGoDemo/internal/domain"
)

// ErrNotFound is returned when the order is not in the map.
var ErrNotFound = errors.New("order not found")

// OrderRepository stores orders in memory.
type OrderRepository struct {
	mu     sync.Mutex
	orders map[string]domain.Order
}

// NewOrderRepository returns an empty in-memory repository.
func NewOrderRepository() *OrderRepository {
	return &OrderRepository{orders: make(map[string]domain.Order)}
}

// Save stores an order.
func (r *OrderRepository) Save(order domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orders[order.ID] = order
	return nil
}

// ByID returns an order.
func (r *OrderRepository) ByID(id string) (domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	order, ok := r.orders[id]
	if !ok {
		return domain.Order{}, ErrNotFound
	}
	return order, nil
}

// Compile-time proof that the adapter implements the port.
var _ application.OrderRepository = (*OrderRepository)(nil)
