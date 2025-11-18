package main

// POST /products – Create a new product in the backend.
// GET /products/list -  For listing all products
// GET /cart/list - For listing cart products
// POST /cart/add - To add  product in cart
// POST /cart/remove - To remove product from cart
// POST /checkout/order - For a checkout

// --- EMBED MIGRATIONS ---
import (
	"database/sql"
	_ "embed"
	"inventory-management/handler"
	"inventory-management/service"
	"inventory-management/store"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

//go:embed migrations.sql
var migrationSQL string

func main() {
	// --- HARD-CODED POSTGRES DSN ---
	dsn := "postgres://postgres:password@localhost:5432/interview?sslmode=disable"

	// Connect to DB
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("DB connection failed: %v", err)
	}
	defer db.Close()

	// --- RUN MIGRATIONS ---
	if _, err := db.Exec(migrationSQL); err != nil {
		log.Fatalf("Failed running migrations: %v", err)
	}
	log.Println("Database migrations executed successfully ✔")

	// --- Store ---
	st := &store.PostgresStore{DB: db}

	// --- Service ---
	svc := service.NewService(st)
	var serviceInterface service.ServiceInterface = svc

	// --- Handlers ---
	h := handler.NewHandler(serviceInterface)

	// --- Router ---
	r := mux.NewRouter()
	h.RegisterRoutes(r)

	// --- Server ---
	log.Println("Server running on :8082")

	if err := http.ListenAndServe(":8082", r); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
