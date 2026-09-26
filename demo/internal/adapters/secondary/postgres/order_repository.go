// Package postgres is a secondary (driven) adapter: it implements the port the
// application layer defined and stores orders in a database. It knows nothing
// about the domain's rules beyond the port it was handed.
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/LukasNiessen/ArchUnitGoDemo/internal/application"
	"github.com/LukasNiessen/ArchUnitGoDemo/internal/domain"
)

// ErrNotFound is returned when the order is not in the database.
var ErrNotFound = errors.New("order not found")

// OrderRepository stores orders in Postgres.
type OrderRepository struct {
	db *sql.DB
}

// NewOrderRepository wires the repository to a database handle.
func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Save inserts an order.
func (r *OrderRepository) Save(order domain.Order) error {
	_, err := r.db.ExecContext(context.Background(),
		`INSERT INTO orders (id, customer_id, amount, status) VALUES ($1, $2, $3, $4)`,
		order.ID, order.CustomerID, order.Amount, order.Status)
	return err
}

// ByID fetches an order.
func (r *OrderRepository) ByID(id string) (domain.Order, error) {
	row := r.db.QueryRowContext(context.Background(),
		`SELECT id, customer_id, amount, status FROM orders WHERE id = $1`, id)
	var order domain.Order
	if err := row.Scan(&order.ID, &order.CustomerID, &order.Amount, &order.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Order{}, ErrNotFound
		}
		return domain.Order{}, err
	}
	return order, nil
}

// Compile-time proof that the adapter implements the port.
var _ application.OrderRepository = (*OrderRepository)(nil)
