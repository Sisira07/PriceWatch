# PriceWatch Frontend

A minimal, backend-focused frontend for visualizing and testing the
PriceWatch API. Plain HTML/CSS/JavaScript — no build step, no framework,
no npm install.

## Pages

- **`index.html`** — Login / Register.
- **`dashboard.html`** — List your watched items, add a new one, and
  trigger an on-demand price check ("Simulate price check now").
- **`item.html`** — One item's detail: edit/delete it, see its full price
  history, and see every notification it's generated.

## Running it

1. Make sure the backend is running (`docker compose up` from the project
   root, or `go run ./cmd/api` locally) and reachable at
   `http://localhost:8080`. If your API runs on a different host/port,
   edit `API_BASE` at the top of `js/api.js`.
2. Serve this folder with any static file server — for example:
   ```
   cd frontend
   python -m http.server 5500
   ```
   Then open `http://localhost:5500` in your browser.

   Opening `index.html` directly by double-clicking it also works in most
   browsers, but a real static server avoids occasional browser quirks
   with `file://` URLs, so it's the more reliable option.
3. Register an account, then explore the dashboard.

## What "Simulate price check now" does

The backend worker normally checks prices automatically every 60 seconds
in the background. Waiting a full minute to see something happen makes for
a slow demo, so both the dashboard and item pages have a button that calls
a small admin endpoint (`POST /admin/simulate-tick`) which runs that exact
same check logic immediately, on demand. It's for testing/demo purposes
only — a real product wouldn't expose this to end users.

## How it's organized

- `js/api.js` — the only file that knows about HTTP and JWTs. Every other
  script calls functions from here instead of using `fetch()` directly.
- `js/ui.js` — small shared helpers (toast messages, formatting, auth
  guard) used by both `dashboard.js` and `item.js`.
- `js/auth.js`, `js/dashboard.js`, `js/item.js` — one script per page,
  containing only that page's logic.
- `css/style.css` — one shared stylesheet for all three pages.

## Notes

- The JWT is stored in `localStorage` for simplicity. That's fine for a
  local demo project; a production app would want something more
  deliberate (short-lived tokens, refresh flow, etc. — see the main
  README's "Future improvements" section).
- CORS is wide open (`Access-Control-Allow-Origin: *`) on the backend to
  make local development friction-free. Restrict this to your real
  frontend's origin before deploying anywhere.
