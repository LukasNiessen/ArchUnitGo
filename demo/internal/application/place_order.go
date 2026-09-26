// Package application is the use-case layer: it orchestrates the domain and
// defines the ports. It may depend on the domain and on nothing in the adapters.
package application

import "github.com/LukasNiessen/ArchUnitGoDemo/internal/domain"

// OrderRepository is the driven port: the hole the core leaves for somebody who
// knows where orders live. An adapter on the secondary side implements it.
type OrderRepository interface {
	Save(order domain.Order) error
	ByID(id string) (domain.Order, error)
}

// PlaceOrder is a use case, the driving port of the application layer. It is the
// one thing an adapter on the primary side is allowed to call.
type PlaceOrder struct {
	Orders OrderRepository
}

// Handle places an order and stores it.
func (uc PlaceOrder) Handle(id, customerID string, amount int64) (domain.Order, error) {
	order := (domain.OrderService{}).Place(domain.Order{
		ID:         id,
		CustomerID: customerID,
		Amount:     amount,
	})
	if err := uc.Orders.Save(order); err != nil {
		return domain.Order{}, err
	}
	return order, nil
}
