package service

import (
	"errors"
	"fmt"
	"inventory-management/store"
	"time"
)

type Service struct {
	store store.Store
}

func NewService(s store.Store) *Service {
	return &Service{store: s}
}

func (s *Service) CreateProduct(name, desc string, price float64) (int64, error) {
	if name == "" {
		return 0, errors.New("name required")
	}
	if price < 0 {
		return 0, errors.New("price must be >= 0")
	}
	return s.store.CreateProduct(name, desc, price)
}

func (s *Service) ListProducts() ([]ProductDTO, error) {
	rows, err := s.store.ListProducts()
	if err != nil {
		return nil, err
	}
	out := make([]ProductDTO, 0, len(rows))
	for _, r := range rows {
		p := ProductDTO{
			ID:          r.ID,
			Name:        r.Name,
			Description: "",
			Price:       r.Price,
		}
		if r.Description.Valid {
			p.Description = r.Description.String
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Service) AddToCart(userID string, productID int64, qty int) error {
	if userID == "" {
		return errors.New("user_id required")
	}
	if qty <= 0 {
		return errors.New("quantity must be > 0")
	}
	return s.store.AddToCart(userID, productID, qty)
}

func (s *Service) RemoveFromCart(userID string, productID int64) error {
	if userID == "" {
		return errors.New("user_id required")
	}
	return s.store.RemoveFromCart(userID, productID)
}

func (s *Service) GetCart(userID string) ([]CartDTO, float64, error) {
	if userID == "" {
		return nil, 0, errors.New("user_id required")
	}
	rows, err := s.store.GetCart(userID)
	if err != nil {
		return nil, 0, err
	}

	// need product prices; for simplicity, call ListProducts and build map
	products, err := s.store.ListProducts()
	if err != nil {
		return nil, 0, err
	}
	priceMap := map[int64]float64{}
	for _, p := range products {
		priceMap[p.ID] = p.Price
	}

	var total float64
	out := make([]CartDTO, 0, len(rows))
	for _, r := range rows {
		price, ok := priceMap[r.ProductID]
		if !ok {
			return nil, 0, fmt.Errorf("product %d not found", r.ProductID)
		}
		out = append(out, CartDTO{ProductID: r.ProductID, Quantity: r.Quantity, Price: price})
		total += price * float64(r.Quantity)
	}
	return out, total, nil
}

func (s *Service) Checkout(userID string) (OrderDTO, error) {
	if userID == "" {
		return OrderDTO{}, errors.New("user_id required")
	}
	orderRow, items, err := s.store.Checkout(userID)
	if err != nil {
		return OrderDTO{}, err
	}
	od := OrderDTO{
		ID:        orderRow.ID,
		UserID:    orderRow.UserID,
		Total:     orderRow.Total,
		CreatedAt: time.Now(),
		Items:     make([]CartDTO, 0, len(items)),
	}
	for _, it := range items {
		od.Items = append(od.Items, CartDTO{ProductID: it.ProductID, Quantity: it.Quantity, Price: it.Price})
	}
	return od, nil
}

func (s *Service) UpdateStock(productID int64, newStock int) error {
	if newStock < 0 {
		return errors.New("stock cannot be negative")
	}
	return s.store.UpdateStock(productID, newStock)
}

// DTOs
type ProductDTO struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
}

type CartDTO struct {
	ProductID int64   `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type OrderDTO struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Items     []CartDTO `json:"items"`
	Total     float64   `json:"total"`
	CreatedAt time.Time `json:"created_at"`
}
