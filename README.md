# 🛒 Shop Demo — Fullstack App (Go + Postgres + React)

This project is a simple **Inventory + Cart + Checkout** application built with:

- **Go backend** (REST API + Postgres)
- **React + Vite frontend**
- **PostgreSQL database**
- Handles **stock**, **cart**, **checkout**, and safe **inventory updates**

This README contains **step-by-step instructions** to run backend and frontend.

---

## 🚀 Features

### Backend
- Product listing  
- Add/remove items from cart  
- Checkout with stock validation  
- Mutex + Postgres row-level locking  
- Clean 3-layer architecture (handlers → service → store)

### Frontend
- View products  
- Add to cart  
- View cart  
- Checkout with shipping details  
- Redirects between /products, /cart, /checkout  

---

# 🗄️ 1. Run the Backend (Go + Postgres)

### **Step 1 — Start PostgreSQL**

Using Docker (recommended):

```bash
docker run --rm -d \
  --name shop-postgres \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=shopdb \
  -p 5432:5432 \
  postgres:15
```

### **Step 2 — Apply DB Migrations**

Copy your migrations.sql:
```
docker cp migrations.sql shop-postgres:/migrations.sql
```

Run migrations:
```
docker exec -it shop-postgres psql -U postgres -d shopdb -f /migrations.sql
```

### **Step 3 — Run Backend**

Inside your Go backend project folder:
```
export DATABASE_DSN="postgres://postgres:password@localhost:5432/shopdb?sslmode=disable"
export LISTEN_ADDR=":8082"

go run .
```

Backend now runs at:
```
http://localhost:8082
```

# 💻 2. Run the Frontend (React + Vite)
*Step 1* — Install frontend dependencies
```
cd my-shop-frontend
npm install
```
*Step 2* — Start frontend server
```
npm run dev
```

Frontend runs at:
```
http://localhost:5173
```
🔗 API Endpoints

|Method |	Endpoint |	Description|
|--------|----------------------------------|-------------------|
|GET	|/products/list |	List all products|
|POST |	/products	| Create product|
|POST |	/cart/add	| Add item to cart|
|POST |	/cart/remove	| Remove item|
|GET	|/cart/list?user_id=demo_user | Get cart|
|POST |	/checkout/order	| Place order|
