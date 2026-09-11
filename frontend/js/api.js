// api.js — the ONLY file that knows about HTTP/JWT details. Every page
// script calls the functions here instead of using fetch() directly, so
// the backend integration lives in exactly one place.

// Change this if your API runs on a different host/port. This matches the
// docker-compose.yml default and the API_ADDR in .env.example.
const API_BASE = "https://pricewatch-api-chrt.onrender.com";

const TOKEN_KEY = "pricewatch_token";

function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token);
}

function clearToken() {
  localStorage.removeItem(TOKEN_KEY);
}

function isLoggedIn() {
  return !!getToken();
}

// Core request helper. Throws an Error with a readable message on any
// non-2xx response, so callers can just try/catch.
async function apiFetch(path, { method = "GET", body, auth = true } = {}) {
  const headers = { "Content-Type": "application/json" };
  if (auth) {
    const token = getToken();
    if (token) headers["Authorization"] = "Bearer " + token;
  }

  let res;
  try {
    res = await fetch(API_BASE + path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch (networkErr) {
    throw new Error(
      "Could not reach the API at " + API_BASE + ". Is the backend running? (" + networkErr.message + ")"
    );
  }

  // No content (e.g. DELETE) — nothing to parse.
  if (res.status === 204) return null;

  let data = null;
  const text = await res.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      // Non-JSON response body; leave data as null.
    }
  }

  if (!res.ok) {
    const message = (data && data.error) || res.statusText || "Request failed";
    throw new Error(message);
  }

  return data;
}

// --- Auth ---

function register(email, password) {
  return apiFetch("/auth/register", { method: "POST", body: { email, password }, auth: false });
}

async function login(email, password) {
  const data = await apiFetch("/auth/login", { method: "POST", body: { email, password }, auth: false });
  setToken(data.token);
  return data;
}

function logout() {
  clearToken();
}

// --- Items ---

function listItems() {
  return apiFetch("/items");
}

function createItem({ name, url, target_price }) {
  return apiFetch("/items", { method: "POST", body: { name, url, target_price } });
}

function getItem(id) {
  return apiFetch("/items/" + encodeURIComponent(id));
}

function updateItem(id, patch) {
  return apiFetch("/items/" + encodeURIComponent(id), { method: "PATCH", body: patch });
}

function deleteItem(id) {
  return apiFetch("/items/" + encodeURIComponent(id), { method: "DELETE" });
}

function getItemHistory(id) {
  return apiFetch("/items/" + encodeURIComponent(id) + "/history");
}

function getItemNotifications(id) {
  return apiFetch("/items/" + encodeURIComponent(id) + "/notifications");
}

// --- Admin / demo ---

function simulateTick() {
  return apiFetch("/admin/simulate-tick", { method: "POST" });
}
