package store

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// ProductRow, CartRow, OrderRow etc are simple structs representing DB rows
type ProductRow struct {
	ID          int64
	Name        string
	Description sql.NullString
	Price       float64
	Stock       int
}

type CartRow struct {
	ProductID int64
	Quantity  int
}

type OrderRow struct {
	ID        int64
	UserID    string
	Total     float64
	CreatedAt time.Time
}

type OrderItemRow struct {
	ProductID int64
	Quantity  int
	Price     float64
}

// PostgresStore is a Store backed by Postgres and has in-process locks
type PostgresStore struct {
	DB *sql.DB

	// per-user mutexes to avoid concurrent goroutines in this process
	// racing on the same cart. Keys are user_id -> *sync.Mutex
	locks sync.Map // map[string]*sync.Mutex

	// (optional) you could add productLocks sync.Map if you want per-product in-process locking
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	DB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := DB.Ping(); err != nil {
		return nil, err
	}
	return &PostgresStore{DB: DB}, nil
}

func (s *PostgresStore) Close() error { return s.DB.Close() }

// helper: acquire per-user lock (process-local). Returns unlock func.
func (s *PostgresStore) lockForUser(userID string) func() {
	// fast path Load
	if v, ok := s.locks.Load(userID); ok {
		m := v.(*sync.Mutex)
		m.Lock()
		return func() { m.Unlock() }
	}

	// Otherwise create and store a new mutex (race-safe via LoadOrStore)
	m := &sync.Mutex{}
	actual, _ := s.locks.LoadOrStore(userID, m)
	mtx := actual.(*sync.Mutex)
	mtx.Lock()
	return func() { mtx.Unlock() }
}

// CreateProduct inserts a product and returns its id
func (s *PostgresStore) CreateProduct(name, desc string, price float64) (int64, error) {
	var id int64
	err := s.DB.QueryRow(
		`INSERT INTO products (name, description, price) VALUES ($1, $2, $3) RETURNING id`,
		name, desc, price,
	).Scan(&id)
	return id, err
}

func (s *PostgresStore) ListProducts() ([]ProductRow, error) {
	rows, err := s.DB.Query(`SELECT id, name, description, price, stock FROM products ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProductRow{}
	for rows.Next() {
		var p ProductRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *PostgresStore) AddToCart(userID string, productID int64, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be > 0")
	}

	// process-local lock to avoid concurrent goroutines in same process
	unlock := s.lockForUser(userID)
	defer unlock()

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	// ensure rollback on early return
	rolledBack := false
	defer func() {
		if !rolledBack {
			_ = tx.Rollback()
		}
	}()

	// ensure cart exists
	if _, err := tx.Exec(`INSERT INTO carts (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}

	// Lock the product row and read stock
	var stock int
	if err := tx.QueryRow(`SELECT stock FROM products WHERE id = $1 FOR UPDATE`, productID).Scan(&stock); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}

	if stock < qty {
		_ = tx.Rollback()
		rolledBack = true
		return ErrInsufficientStock
	}

	// Upsert cart item (add quantity)
	if _, err := tx.Exec(`
		INSERT INTO cart_items (cart_id, product_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (cart_id, product_id)
		DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
	`, userID, productID, qty); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}

	// decrement product stock (reserved)
	if _, err := tx.Exec(`UPDATE products SET stock = stock - $1 WHERE id = $2`, qty, productID); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}
	rolledBack = true
	return nil
}

func (s *PostgresStore) RemoveFromCart(userID string, productID int64) error {
	// process-local lock
	unlock := s.lockForUser(userID)
	defer unlock()

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	rolledBack := false
	defer func() {
		if !rolledBack {
			_ = tx.Rollback()
		}
	}()

	// read current quantity in cart
	var qty int
	err = tx.QueryRow(`SELECT quantity FROM cart_items WHERE cart_id=$1 AND product_id=$2`, userID, productID).Scan(&qty)
	if err == sql.ErrNoRows {
		_ = tx.Rollback()
		rolledBack = true
		return sql.ErrNoRows
	}
	if err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}

	// delete item from cart
	if _, err := tx.Exec(`DELETE FROM cart_items WHERE cart_id=$1 AND product_id=$2`, userID, productID); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}

	// restore reserved stock
	if _, err := tx.Exec(`UPDATE products SET stock = stock + $1 WHERE id = $2`, qty, productID); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return err
	}
	rolledBack = true
	return nil
}


func (s *PostgresStore) GetCart(userID string) ([]CartRow, error) {
	rows, err := s.DB.Query(`SELECT product_id, quantity FROM cart_items WHERE cart_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CartRow{}
	for rows.Next() {
		var c CartRow
		if err := rows.Scan(&c.ProductID, &c.Quantity); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Checkout when stock was already reserved on AddToCart.
// Creates order + order_items and clears the cart. Does NOT modify products.stock.
func (s *PostgresStore) Checkout(userID string) (OrderRow, []OrderItemRow, error) {
	var order OrderRow
	var items []OrderItemRow

	// process-local lock (optional extra safety)
	unlock := s.lockForUser(userID)
	defer unlock()

	tx, err := s.DB.Begin()
	if err != nil {
		return order, items, err
	}
	// ensure rollback on any early return
	rolledBack := false
	defer func() {
		if !rolledBack {
			_ = tx.Rollback()
		}
	}()

	// Read cart items and lock product rows defensively (ORDER BY to avoid deadlocks)
	rows, err := tx.Query(`
		SELECT ci.product_id, ci.quantity, p.price
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.cart_id = $1
		ORDER BY p.id
		FOR UPDATE
	`, userID)
	if err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return order, items, err
	}
	defer rows.Close()

	var total float64
	for rows.Next() {
		var it OrderItemRow
		if err := rows.Scan(&it.ProductID, &it.Quantity, &it.Price); err != nil {
			_ = tx.Rollback()
			rolledBack = true
			return order, items, err
		}
		items = append(items, it)
		total += float64(it.Quantity) * it.Price
	}
	if len(items) == 0 {
		_ = tx.Rollback()
		rolledBack = true
		return order, items, errors.New("cart empty")
	}

	// Create order and get id
	var orderID int64
	var createdAt time.Time
	if err := tx.QueryRow(`INSERT INTO orders (user_id, total) VALUES ($1,$2) RETURNING id, created_at`, userID, total).Scan(&orderID, &createdAt); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return order, items, err
	}

	// Insert order_items
	stmt, err := tx.Prepare(`INSERT INTO order_items (order_id, product_id, quantity, price) VALUES ($1,$2,$3,$4)`)
	if err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return order, items, err
	}
	defer stmt.Close()

	for _, it := range items {
		if _, err := stmt.Exec(orderID, it.ProductID, it.Quantity, it.Price); err != nil {
			_ = tx.Rollback()
			rolledBack = true
			return order, items, err
		}
	}

	// Clear cart (stock already reserved earlier at AddToCart)
	if _, err := tx.Exec(`DELETE FROM cart_items WHERE cart_id = $1`, userID); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return order, items, err
	}
	if _, err := tx.Exec(`DELETE FROM carts WHERE user_id = $1`, userID); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return order, items, err
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		rolledBack = true
		return order, items, err
	}
	rolledBack = true

	order = OrderRow{ID: orderID, UserID: userID, Total: total, CreatedAt: createdAt}
	return order, items, nil
}
