package handler

import (
	"database/sql"
	"encoding/json"
	"inventory-management/service"
	"net/http"

	"github.com/gorilla/mux"
)

// Handler is the HTTP layer that talks to service.Service
type Handler struct {
	svc service.ServiceInterface
}

// NewHandler returns a Handler instance
func NewHandler(s service.ServiceInterface) *Handler {
	return &Handler{svc: s}
}

// RegisterRoutes registers all routes on the provided router
func (h *Handler) RegisterRoutes(r *mux.Router) {
	// Products
	r.HandleFunc("/products", h.CreateProduct).Methods("POST")
	r.HandleFunc("/products/list", h.ListProducts).Methods("GET")

	// Cart
	r.HandleFunc("/cart/add", h.AddToCart).Methods("POST")
	r.HandleFunc("/cart/remove", h.RemoveFromCart).Methods("POST")
	r.HandleFunc("/cart/list", h.ListCart).Methods("GET")

	// Checkout
	r.HandleFunc("/checkout/order", h.Checkout).Methods("POST")
}

// --- request / response shapes ---
type createProductReq struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Price       float64 `json:"price"`
}

type updateStockReq struct {
	ProductID int64 `json:"product_id"`
	NewStock  int   `json:"new_stock"`
}

type addRemoveCartReq struct {
	UserID    string `json:"user_id"`
	ProductID int64  `json:"product_id"`
	Quantity  int    `json:"quantity,omitempty"` // optional for remove
}

// --- helpers ---
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// --- Handler ---

// CreateProduct handles POST /products
func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Price < 0 {
		writeErr(w, http.StatusBadRequest, "price must be >= 0")
		return
	}

	id, err := h.svc.CreateProduct(req.Name, req.Description, req.Price)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// ListProducts handles GET /products/list
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	ps, err := h.svc.ListProducts()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

// AddToCart handles POST /cart/add
// body: { "user_id": "...", "product_id": 1, "quantity": 2 }
func (h *Handler) AddToCart(w http.ResponseWriter, r *http.Request) {
	var req addRemoveCartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.UserID == "" {
		writeErr(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Quantity <= 0 {
		writeErr(w, http.StatusBadRequest, "quantity must be > 0")
		return
	}
	if err := h.svc.AddToCart(req.UserID, req.ProductID, req.Quantity); err != nil {
		// service returns descriptive errors; map them to HTTP codes if needed
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

// RemoveFromCart handles POST /cart/remove
// body: { "user_id": "...", "product_id": 1 }
func (h *Handler) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	var req addRemoveCartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.UserID == "" {
		writeErr(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if err := h.svc.RemoveFromCart(req.UserID, req.ProductID); err != nil {
		// If store returns sql.ErrNoRows, you might map to 404 — here we return 400 for simplicity
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// ListCart handles GET /cart/list?user_id=...
func (h *Handler) ListCart(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeErr(w, http.StatusBadRequest, "user_id required")
		return
	}
	items, total, err := h.svc.GetCart(userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user_id": userID, "items": items, "total": total})
}

// Checkout handles POST /checkout/order
// body: { "user_id": "..." }
func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.UserID == "" {
		writeErr(w, http.StatusBadRequest, "user_id required")
		return
	}
	ord, err := h.svc.Checkout(req.UserID)
	if err != nil {
		// possible errors: cart empty, product missing, DB problems
		// map known errors to appropriate codes as needed
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ord)
}

func (h *Handler) UpdateStock(w http.ResponseWriter, r *http.Request) {
	var req updateStockReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.ProductID == 0 {
		writeErr(w, http.StatusBadRequest, "product_id required")
		return
	}
	if req.NewStock < 0 {
		writeErr(w, http.StatusBadRequest, "new_stock must be >= 0")
		return
	}
	if err := h.svc.UpdateStock(req.ProductID, req.NewStock); err != nil {
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusNotFound, "product not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
