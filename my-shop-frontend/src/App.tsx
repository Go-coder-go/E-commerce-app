import React, { useEffect, useState } from "react";
import { BrowserRouter as Router, Routes, Route, Link, Navigate, useNavigate } from "react-router-dom";

// Single-file React app (Tailwind-friendly classes)
// Default export a component. Use this file as src/App.jsx in a create-react-app / Vite project.

export default function App() {
  return (
    <Router>
      <div className="min-h-screen bg-gray-50 text-gray-900">
        <Header />
        <main className="max-w-7xl mx-auto p-6">
          <Routes>
            {/* Redirect short paths to the list routes (client-side) */}
            <Route path="/products" element={<Navigate to="/products/list" replace />} />
            <Route path="/cart" element={<Navigate to="/cart/list" replace />} />

            <Route path="/products/list" element={<ProductsPage />} />
            <Route path="/cart/list" element={<CartPage />} />
            <Route path="/checkout" element={<CheckoutPage />} />

            {/* home -> products */}
            <Route path="/" element={<Navigate to="/products/list" replace />} />

            {/* fallback */}
            <Route path="*" element={<NotFound />} />
          </Routes>
        </main>
      </div>
    </Router>
  );
}

function Header() {
  return (
    <header className="bg-white shadow">
      <div className="max-w-6xl mx-auto px-4 py-4 flex items-center justify-between">
        <h1 className="text-xl font-semibold">Shop Demo</h1>
        <nav className="space-x-4">
          <Link to="/products/list" className="text-blue-600 hover:underline">
            Products
          </Link>
          <Link to="/cart/list" className="text-blue-600 hover:underline">
            Cart
          </Link>
          <Link to="/checkout" className="text-blue-600 hover:underline">
            Checkout
          </Link>
        </nav>
      </div>
    </header>
  );
}

function ProductsPage() {
  const [products, setProducts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    let mounted = true;
    setLoading(true);
    fetch("/products/list")
      .then((r) => {
        if (!r.ok) throw new Error(`status=${r.status}`);
        return r.json();
      })
      .then((data) => {
        if (!mounted) return;
        setProducts(data);
      })
      .catch((err) => setError(err.message))
      .finally(() => mounted && setLoading(false));
    return () => (mounted = false);
  }, []);

  if (loading) return <div className="p-6">Loading products…</div>;
  if (error) return <div className="p-6 text-red-600">Error: {error}</div>;

  return (
    <div>
      <h2 className="text-2xl font-bold mb-4">Products</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {products.map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </div>
    </div>
  );
}

function ProductCard({ product }) {
  const [adding, setAdding] = useState(false);
  const [qty, setQty] = useState(1);

  async function onAdd() {
    setAdding(true);
    try {
      const body = { user_id: "demo_user", product_id: product.id, quantity: Number(qty) };
      const res = await fetch("/cart/add", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) {
        const j = await res.json().catch(() => ({}));
        throw new Error(j.error || `status=${res.status}`);
      }
      // optionally show toast — simple alert for demo
      alert("Added to cart");
    } catch (err) {
      alert("Add failed: " + err.message);
    } finally {
      setAdding(false);
    }
  }

  return (
    <div className="bg-white rounded shadow p-4 flex flex-col">
      <div className="flex-1">
        <h3 className="font-semibold text-lg">{product.name}</h3>
        <p className="text-sm text-gray-600">{product.description}</p>
      </div>
      <div className="mt-4 flex items-center justify-between">
        <div className="text-lg font-bold">₹{product.price}</div>
        <div className="flex items-center gap-2">
          <input
            type="number"
            min={1}
            value={qty}
            onChange={(e) => setQty(Math.max(1, Number(e.target.value) || 1))}
            className="w-20 border rounded px-2 py-1"
          />
          <button
            onClick={onAdd}
            disabled={adding}
            className="bg-blue-600 text-white px-3 py-1 rounded disabled:opacity-60"
          >
            {adding ? "Adding…" : "Add"}
          </button>
        </div>
      </div>
    </div>
  );
}

function CartPage() {
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const navigate = useNavigate();

  async function load() {
    setLoading(true);
    try {
      const res = await fetch(`/cart/list?user_id=demo_user`);
      if (!res.ok) throw new Error(`status=${res.status}`);
      const j = await res.json();
      // expected shape: { user_id, items: [{product_id, quantity, price}], total }
      setItems(j.items || []);
      setTotal(j.total || 0);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { load(); }, []);

  async function onRemove(productID) {
    try {
      const res = await fetch(`/cart/remove`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ user_id: "demo_user", product_id: productID }),
      });
      if (!res.ok) throw new Error(`status=${res.status}`);
      await load();
    } catch (err) {
      alert("Remove failed: " + err.message);
    }
  }

  if (loading) return <div className="p-6">Loading cart…</div>;
  if (error) return <div className="p-6 text-red-600">Error: {error}</div>;

  return (
    <div>
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold">Your Cart</h2>
        <button onClick={() => navigate('/checkout')} className="bg-green-600 text-white px-3 py-1 rounded">Checkout</button>
      </div>

      {items.length === 0 ? (
        <div className="p-6 text-gray-600">Your cart is empty.</div>
      ) : (
        <div className="mt-4 bg-white rounded shadow">
          <table className="w-full table-auto">
            <thead className="text-left">
              <tr className="border-b"><th className="p-3">Product</th><th>Qty</th><th>Price</th><th></th></tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <tr key={it.product_id} className="border-b">
                  <td className="p-3">Product #{it.product_id}</td>
                  <td className="p-3">{it.quantity}</td>
                  <td className="p-3">₹{it.price}</td>
                  <td className="p-3">
                    <button onClick={() => onRemove(it.product_id)} className="text-red-600">Remove</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <div className="p-4 text-right font-semibold">Total: ₹{total}</div>
        </div>
      )}
    </div>
  );
}

function CheckoutPage() {
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [form, setForm] = useState({ name: "", email: "", address: "", city: "", zip: "" });
  const navigate = useNavigate();

  useEffect(() => {
    let mounted = true;
    fetch(`/cart/list?user_id=demo_user`)
      .then((r) => r.json())
      .then((j) => {
        if (!mounted) return;
        setItems(j.items || []); setTotal(j.total || 0);
      })
      .catch((e) => setError(e.message))
      .finally(() => mounted && setLoading(false));
    return () => (mounted = false);
  }, []);

  async function onSubmit(e) {
    e.preventDefault();
    try {
      // In this simple demo we just call checkout endpoint which expects user_id
      const res = await fetch(`/checkout/order`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ user_id: "demo_user" }),
      });
      if (!res.ok) {
        const j = await res.json().catch(() => ({}));
        throw new Error(j.error || `status=${res.status}`);
      }
      const order = await res.json();
      alert("Order placed! ID: " + order.id);
      // client-side navigation (no full page reload)
      navigate("/products/list");
    } catch (err) {
      alert("Checkout failed: " + err.message);
    }
  }

  if (loading) return <div className="p-6">Loading checkout…</div>;
  if (error) return <div className="p-6 text-red-600">Error: {error}</div>;

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <form className="bg-white rounded shadow p-6" onSubmit={onSubmit}>
        <h2 className="text-xl font-semibold mb-4">Shipping & Billing</h2>
        <label className="block mb-2">Full name
          <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="w-full border p-2 rounded mt-1" required /></label>
        <label className="block mb-2">Email
          <input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} className="w-full border p-2 rounded mt-1" required /></label>
        <label className="block mb-2">Address
          <input value={form.address} onChange={(e) => setForm({ ...form, address: e.target.value })} className="w-full border p-2 rounded mt-1" required /></label>
        <div className="flex gap-2">
          <input placeholder="City" value={form.city} onChange={(e) => setForm({ ...form, city: e.target.value })} className="flex-1 border p-2 rounded" required />
          <input placeholder="ZIP" value={form.zip} onChange={(e) => setForm({ ...form, zip: e.target.value })} className="w-28 border p-2 rounded" required />
        </div>
        <button type="submit" className="mt-4 bg-blue-600 text-white px-4 py-2 rounded">Place order</button>
      </form>

      <aside className="bg-white rounded shadow p-6">
        <h3 className="font-semibold text-lg mb-2">Order summary</h3>
        {items.length === 0 ? <div className="text-gray-600">No items selected</div> : (
          <div>
            <ul className="space-y-2">
              {items.map(it => (
                <li key={it.product_id} className="flex justify-between">
                  <span>Product #{it.product_id} x {it.quantity}</span>
                  <span>₹{it.price * it.quantity}</span>
                </li>
              ))}
            </ul>
            <div className="mt-4 font-semibold">Total: ₹{total}</div>
          </div>
        )}
      </aside>
    </div>
  );
}

function NotFound() {
  return (
    <div className="p-6">
      <h2 className="text-xl font-bold">404 — Not found</h2>
      <p className="mt-2">The page you requested does not exist.</p>
    </div>
  );
}

// End of file
