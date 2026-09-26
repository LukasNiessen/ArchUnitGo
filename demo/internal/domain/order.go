// Package domain is the core of the hexagon: the entities and the rules of the
// business, and nothing else. It imports no project code and no third-party
// library — that is exactly what the architecture tests in the architecture
// package enforce.
package domain

// OrderStatus is where an order is in its lifecycle.
type OrderStatus string

const (
	// StatusPlaced is an order that has been created but not yet paid.
	StatusPlaced OrderStatus = "placed"
	// StatusPaid is an order that has been paid.
	StatusPaid OrderStatus = "paid"
	// StatusShipped is an order that has been sent.
	StatusShipped OrderStatus = "shipped"
)

// Order is a purchase a customer has placed.
type Order struct {
	ID         string
	CustomerID string
	Amount     int64
	Status     OrderStatus
}
