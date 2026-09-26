package domain

import "errors"

// ErrNotPlaced is returned when an order that has not been placed is asked to be paid.
var ErrNotPlaced = errors.New("order not placed")

// OrderService holds the rules of an order's lifecycle and no rule about where
// the order is stored or how it was asked for. Those belong to the adapters.
type OrderService struct{}

// Place starts an order's lifecycle.
func (OrderService) Place(order Order) Order {
	order.Status = StatusPlaced
	return order
}

// Pay moves a placed order to paid.
func (OrderService) Pay(order Order) (Order, error) {
	if order.Status != StatusPlaced {
		return Order{}, ErrNotPlaced
	}
	order.Status = StatusPaid
	return order, nil
}
