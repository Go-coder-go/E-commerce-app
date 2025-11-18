package store

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAddToCart_SuccessAndInvalidQty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	s := &PostgresStore{DB: db}

	// invalid qty -> should error early, no DB calls
	if err := s.AddToCart("u1", 1, 0); err == nil {
		t.Fatalf("expected error for qty <= 0")
	}

	// success path: expect insert cart (ON CONFLICT DO NOTHING) then upsert cart_items
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO carts (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`)).
		WithArgs("u1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// upsert cart_items
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO cart_items (cart_id, product_id, quantity) VALUES ($1,$2,$3)
		ON CONFLICT (cart_id, product_id)
		DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity
	`)).
		WithArgs("u1", int64(10), 3).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := s.AddToCart("u1", 10, 3); err != nil {
		t.Fatalf("AddToCart failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestRemoveFromCart_NoRowsAndSuccess(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	s := &PostgresStore{DB: db}

	// no rows affected -> sql.ErrNoRows expected
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM cart_items WHERE cart_id=$1 AND product_id=$2`)).
		WithArgs("u1", int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := s.RemoveFromCart("u1", 5); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}

	// success -> rows affected >0
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM cart_items WHERE cart_id=$1 AND product_id=$2`)).
		WithArgs("u1", int64(5)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := s.RemoveFromCart("u1", 5); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetCart_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	s := &PostgresStore{DB: db}

	rows := sqlmock.NewRows([]string{"product_id", "quantity"}).
		AddRow(int64(11), 2).
		AddRow(int64(12), 1)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT product_id, quantity FROM cart_items WHERE cart_id=$1`)).
		WithArgs("u1").
		WillReturnRows(rows)

	got, err := s.GetCart("u1")
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(got) != 2 || got[0].ProductID != 11 || got[0].Quantity != 2 {
		t.Fatalf("unexpected cart rows: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCheckout_InsufficientStock(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	s := &PostgresStore{DB: db}

	// Begin transaction
	mock.ExpectBegin()
	// Query returns a row with stock < qty -> should return ErrInsufficientStock
	rows := sqlmock.NewRows([]string{"product_id", "quantity", "price", "stock"}).
		AddRow(int64(100), 5, 10.0, 3) // stock 3 < qty 5
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT ci.product_id, ci.quantity, p.price, p.stock
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.cart_id = $1
		FOR UPDATE
	`)).WithArgs("userx").WillReturnRows(rows)

	// Because code returns ErrInsufficientStock before making changes, transaction should be rolled back.
	// The implementation defers rollback if err != nil — sqlmock will accept a rollback call if it happens.
	mock.ExpectRollback()

	_, _, err := s.Checkout("userx")
	if err == nil || !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestCheckout_Success(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	s := &PostgresStore{DB: db}

	// Begin
	mock.ExpectBegin()
	// Query cart items -> two products
	rows := sqlmock.NewRows([]string{"product_id", "quantity", "price", "stock"}).
		AddRow(int64(1), 2, 10.0, 5).
		AddRow(int64(2), 1, 20.0, 3)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT ci.product_id, ci.quantity, p.price, p.stock
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.cart_id = $1
		FOR UPDATE
	`)).WithArgs("userA").WillReturnRows(rows)

	// Validate and insert order -> expect insert returning id,created_at
	createdAt := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO orders (user_id, total) VALUES ($1,$2) RETURNING id, created_at`)).
		WithArgs("userA", 40.0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(77), createdAt))

	// Prepare insert order_items
	mock.ExpectPrepare(regexp.QuoteMeta(`INSERT INTO order_items (order_id, product_id, quantity, price) VALUES ($1,$2,$3,$4)`))
	// Exec insert for first item
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO order_items (order_id, product_id, quantity, price) VALUES ($1,$2,$3,$4)`)).
		WithArgs(int64(77), int64(1), 2, 10.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Exec insert for second item
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO order_items (order_id, product_id, quantity, price) VALUES ($1,$2,$3,$4)`)).
		WithArgs(int64(77), int64(2), 1, 20.0).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Prepare update stock and expect two updates
	mock.ExpectPrepare(regexp.QuoteMeta(`UPDATE products SET stock = stock - $1 WHERE id = $2`))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE products SET stock = stock - $1 WHERE id = $2`)).
		WithArgs(2, int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE products SET stock = stock - $1 WHERE id = $2`)).
		WithArgs(1, int64(2)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Delete cart_items and cart
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM cart_items WHERE cart_id = $1`)).
		WithArgs("userA").
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM carts WHERE user_id = $1`)).
		WithArgs("userA").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Commit
	mock.ExpectCommit()

	order, items, err := s.Checkout("userA")
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}
	if order.ID != 77 || order.UserID != "userA" || len(items) != 2 {
		t.Fatalf("unexpected order result: %+v %+v", order, items)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
