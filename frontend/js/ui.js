// ui.js — tiny shared UI helpers used across dashboard.html and item.html.

let toastTimer = null;

function showToast(message, type = "info") {
  const el = document.getElementById("toast");
  if (!el) return;

  el.textContent = message;
  el.className = "show" + (type === "error" ? " toast-error" : type === "success" ? " toast-success" : "");

  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    el.className = "";
  }, 3200);
}

function formatMoney(value) {
  if (value === null || value === undefined) return "—";
  return "$" + Number(value).toFixed(2);
}

function formatDateTime(iso) {
  if (!iso) return "—";
  const d = new Date(iso);
  return d.toLocaleString();
}

// Redirect to the login page if there's no token. Call at the top of any
// page that requires auth.
function requireAuth() {
  if (!isLoggedIn()) {
    window.location.href = "index.html";
  }
}

function wireLogoutButton() {
  const btn = document.getElementById("logout-btn");
  if (btn) {
    btn.addEventListener("click", () => {
      logout();
      window.location.href = "index.html";
    });
  }
}
