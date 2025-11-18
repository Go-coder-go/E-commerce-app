package service

type ServiceInterface interface {
	CreateProduct(name, desc string, price float64) (int64, error)
	ListProducts() ([]ProductDTO, error)
	AddToCart(userID string, productID int64, qty int) error
	RemoveFromCart(userID string, productID int64) error
	GetCart(userID string) ([]CartDTO, float64, error)
	Checkout(userID string) (OrderDTO, error)
	UpdateStock(productID int64, newStock int) error
}
