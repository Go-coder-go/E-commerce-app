package store

import (
	"database/sql"
	"errors"
)

// ErrInsufficientStock returned when requested qty exceeds available stock.
var ErrInsufficientStock = errors.New("insufficient stock")

// UpdateStock sets the absolute stock for a product (admin operation).
func (s *PostgresStore) UpdateStock(productID int64, newStock int) error {
	if newStock < 0 {
		return errors.New("stock cannot be negative")
	}
	res, err := s.DB.Exec(`UPDATE products SET stock=$1 WHERE id=$2`, newStock, productID)
	if err != nil {
		return err
	}
	ra, _ := res.RowsAffected()
	if ra == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetStock returns current stock for a product.
func (s *PostgresStore) GetStock(productID int64) (int, error) {
	var stock int
	if err := s.DB.QueryRow(`SELECT stock FROM products WHERE id=$1`, productID).Scan(&stock); err != nil {
		return 0, err
	}
	return stock, nil
}
