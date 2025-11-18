package service

import (
	"database/sql"
	"errors"
	"fmt"
	"inventory-management/store"
	"reflect"
	"testing"
	"time"
)

// ---- fakeStore implementing store.Store partially for tests ----
type fakeStore struct {
	CreateProductFn  func(name, desc string, price float64) (int64, error)
	ListProductsFn   func() ([]store.ProductRow, error)
	AddToCartFn      func(userID string, productID int64, qty int) error
	RemoveFromCartFn func(userID string, productID int64) error
	GetCartFn        func(userID string) ([]store.CartRow, error)
	CheckoutFn       func(userID string) (store.OrderRow, []store.OrderItemRow, error)
	UpdateStockFn    func(productID int64, newStock int) error
}

func (f *fakeStore) CreateProduct(name, desc string, price float64) (int64, error) {
	return f.CreateProductFn(name, desc, price)
}
func (f *fakeStore) ListProducts() ([]store.ProductRow, error) { return f.ListProductsFn() }
func (f *fakeStore) AddToCart(userID string, productID int64, qty int) error {
	return f.AddToCartFn(userID, productID, qty)
}
func (f *fakeStore) RemoveFromCart(userID string, productID int64) error {
	return f.RemoveFromCartFn(userID, productID)
}
func (f *fakeStore) GetCart(userID string) ([]store.CartRow, error) { return f.GetCartFn(userID) }
func (f *fakeStore) Checkout(userID string) (store.OrderRow, []store.OrderItemRow, error) {
	return f.CheckoutFn(userID)
}
func (f *fakeStore) UpdateStock(productID int64, newStock int) error {
	return f.UpdateStockFn(productID, newStock)
}
func (f *fakeStore) Close() error { return nil }

// ---- Tests ----

func TestCreateProductValidationAndForwarding(t *testing.T) {
	svc := NewService(&fakeStore{
		CreateProductFn: func(name, desc string, price float64) (int64, error) {
			return 123, nil
		},
	})

	// name empty -> error
	if _, err := svc.CreateProduct("", "d", 10); err == nil {
		t.Fatalf("expected error for empty name")
	}

	// negative price -> error
	if _, err := svc.CreateProduct("n", "d", -1); err == nil {
		t.Fatalf("expected error for negative price")
	}

	// OK path -> forwards to store
	id, err := svc.CreateProduct("n", "desc", 12.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 123 {
		t.Fatalf("expected id 123, got %d", id)
	}
}

func TestListProductsMapping(t *testing.T) {
	sRows := []store.ProductRow{
		{
			ID:          1,
			Name:        "p1",
			Description: sql.NullString{String: "d1", Valid: true},
			Price:       99.5,
		},
		{
			ID:          2,
			Name:        "p2",
			Description: sql.NullString{Valid: false},
			Price:       10.0,
		},
	}
	svc := NewService(&fakeStore{
		ListProductsFn: func() ([]store.ProductRow, error) { return sRows, nil },
	})

	out, err := svc.ListProducts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 products, got %d", len(out))
	}
	if out[0].Description != "d1" {
		t.Fatalf("expected desc 'd1' for first product, got %q", out[0].Description)
	}
	if out[1].Description != "" {
		t.Fatalf("expected empty desc for second product, got %q", out[1].Description)
	}
}

func TestAddToCartValidationAndForwarding(t *testing.T) {
	called := false
	fs := &fakeStore{
		AddToCartFn: func(userID string, productID int64, qty int) error {
			called = true
			if userID == "" || qty <= 0 {
				return errors.New("invalid")
			}
			return nil
		},
	}
	svc := NewService(fs)

	// missing user
	if err := svc.AddToCart("", 1, 1); err == nil {
		t.Fatalf("expected error for missing user")
	}

	// invalid qty
	if err := svc.AddToCart("u", 1, 0); err == nil {
		t.Fatalf("expected error for qty <= 0")
	}

	// ok
	if err := svc.AddToCart("u", 1, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected store.AddToCart to be called")
	}
}

func TestRemoveFromCartValidationAndForwarding(t *testing.T) {
	called := false
	fs := &fakeStore{
		RemoveFromCartFn: func(userID string, productID int64) error {
			called = true
			if userID == "" {
				return errors.New("bad")
			}
			return nil
		},
	}
	svc := NewService(fs)

	if err := svc.RemoveFromCart("", 1); err == nil {
		t.Fatalf("expected error for empty user")
	}
	if err := svc.RemoveFromCart("u", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected store.RemoveFromCart to be called")
	}
}

func TestGetCartSuccessAndMissingProduct(t *testing.T) {
	// Setup fake store: GetCart returns one cart row; ListProducts returns the corresponding product price
	fs := &fakeStore{
		GetCartFn: func(userID string) ([]store.CartRow, error) {
			return []store.CartRow{{ProductID: 101, Quantity: 2}}, nil
		},
		ListProductsFn: func() ([]store.ProductRow, error) {
			return []store.ProductRow{{ID: 101, Name: "p", Description: sql.NullString{Valid: false}, Price: 50.0}}, nil
		},
	}
	svc := NewService(fs)

	items, total, err := svc.GetCart("u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if total != 100.0 {
		t.Fatalf("expected total 100.0, got %v", total)
	}

	// missing product case
	fs2 := &fakeStore{
		GetCartFn: func(userID string) ([]store.CartRow, error) {
			return []store.CartRow{{ProductID: 202, Quantity: 1}}, nil
		},
		ListProductsFn: func() ([]store.ProductRow, error) {
			return []store.ProductRow{}, nil
		},
	}
	svc2 := NewService(fs2)
	_, _, err = svc2.GetCart("u2")
	if err == nil {
		t.Fatalf("expected error for missing product")
	}
}

func TestCheckoutFlow(t *testing.T) {
	// empty user validation
	svc := NewService(&fakeStore{})
	if _, err := svc.Checkout(""); err == nil {
		t.Fatalf("expected error for empty user")
	}

	// success case: store.Checkout returns order row and order items
	fs := &fakeStore{
		CheckoutFn: func(userID string) (store.OrderRow, []store.OrderItemRow, error) {
			return store.OrderRow{ID: 55, UserID: userID, Total: 200.0, CreatedAt: time.Now()},
				[]store.OrderItemRow{{ProductID: 11, Quantity: 2, Price: 100.0}},
				nil
		},
	}
	svc2 := NewService(fs)
	od, err := svc2.Checkout("u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if od.ID != 55 || od.UserID != "u1" || od.Total != 200.0 {
		t.Fatalf("unexpected order dto: %+v", od)
	}
	if len(od.Items) != 1 || od.Items[0].ProductID != 11 || od.Items[0].Quantity != 2 {
		t.Fatalf("unexpected items mapping: %+v", od.Items)
	}

	// store error propagation
	fs2 := &fakeStore{
		CheckoutFn: func(userID string) (store.OrderRow, []store.OrderItemRow, error) {
			return store.OrderRow{}, nil, errors.New("db err")
		},
	}
	svc3 := NewService(fs2)
	if _, err := svc3.Checkout("u1"); err == nil {
		t.Fatalf("expected error from store to propagate")
	}
}

func TestUpdateStockValidationAndForwarding(t *testing.T) {
	// negative newStock validation
	svc := NewService(&fakeStore{})
	if err := svc.UpdateStock(1, -5); err == nil {
		t.Fatalf("expected error for negative stock")
	}

	called := false
	fs := &fakeStore{
		UpdateStockFn: func(productID int64, newStock int) error {
			called = true
			if productID != 7 || newStock != 10 {
				return fmt.Errorf("unexpected args")
			}
			return nil
		},
	}
	svc2 := NewService(fs)
	if err := svc2.UpdateStock(7, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected UpdateStock to call store")
	}
}

// Extra: test ListProducts forwarding error
func TestListProductsStoreError(t *testing.T) {
	fs := &fakeStore{
		ListProductsFn: func() ([]store.ProductRow, error) { return nil, errors.New("db down") },
	}
	svc := NewService(fs)
	if _, err := svc.ListProducts(); err == nil {
		t.Fatalf("expected store error to propagate")
	}
}

// Extra: test GetCart store error propagation
func TestGetCartStoreError(t *testing.T) {
	fs := &fakeStore{
		GetCartFn: func(userID string) ([]store.CartRow, error) { return nil, errors.New("db fail") },
	}
	svc := NewService(fs)
	if _, _, err := svc.GetCart("u"); err == nil {
		t.Fatalf("expected error from GetCart to propagate")
	}
}

// Extra: test AddToCart store error propagation
func TestAddToCartStoreError(t *testing.T) {
	fs := &fakeStore{
		AddToCartFn: func(userID string, productID int64, qty int) error { return errors.New("insufficient") },
	}
	svc := NewService(fs)
	if err := svc.AddToCart("u", 1, 1); err == nil {
		t.Fatalf("expected store error to propagate")
	}
}

// Extra: test RemoveFromCart store error propagation
func TestRemoveFromCartStoreError(t *testing.T) {
	fs := &fakeStore{
		RemoveFromCartFn: func(userID string, productID int64) error { return errors.New("no row") },
	}
	svc := NewService(fs)
	if err := svc.RemoveFromCart("u", 1); err == nil {
		t.Fatalf("expected store error to propagate")
	}
}

// Utility: ensure struct equality of DTOs produced (sanity)
func TestProductDTOEquality(t *testing.T) {
	fs := &fakeStore{
		ListProductsFn: func() ([]store.ProductRow, error) {
			return []store.ProductRow{
				{ID: 10, Name: "x", Description: sql.NullString{String: "d", Valid: true}, Price: 1.5},
			}, nil
		},
	}
	svc := NewService(fs)
	out, err := svc.ListProducts()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []ProductDTO{{ID: 10, Name: "x", Description: "d", Price: 1.5}}
	// ignore CreatedAt in comparison
	for i := range out {
		out[i].CreatedAt = time.Time{}
	}
	if !reflect.DeepEqual(out, expected) {
		t.Fatalf("unexpected mapping. got %+v, want %+v", out, expected)
	}
}
