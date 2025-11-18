package store

// POST /products – Create a new product in the backend.
// GET /products/list -  For listing all products
// GET /cart/list - For listing cart products
// POST /cart/add - To add  product in cart
// POST /cart/remove - To remove product from cart
// POST /checkout/order - For a checkout

type Store interface {
	CreateProduct(name, desc string, price float64) (int64, error)
	ListProducts() ([]ProductRow, error)

	AddToCart(userID string, productID int64, qty int) error
	RemoveFromCart(userID string, productID int64) error
	GetCart(userID string) ([]CartRow, error)

	Checkout(userID string) (OrderRow, []OrderItemRow, error)
	UpdateStock(productID int64, newStock int) error

	Close() error
}
