// auth.js — logic for index.html (login/register).

// If already logged in, skip straight to the dashboard.
if (isLoggedIn()) {
  window.location.href = "dashboard.html";
}

const tabLogin = document.getElementById("tab-login");
const tabRegister = document.getElementById("tab-register");
const loginForm = document.getElementById("login-form");
const registerForm = document.getElementById("register-form");

function showLoginTab() {
  tabLogin.classList.add("active");
  tabRegister.classList.remove("active");
  loginForm.style.display = "block";
  registerForm.style.display = "none";
}

function showRegisterTab() {
  tabRegister.classList.add("active");
  tabLogin.classList.remove("active");
  registerForm.style.display = "block";
  loginForm.style.display = "none";
}

tabLogin.addEventListener("click", showLoginTab);
tabRegister.addEventListener("click", showRegisterTab);

loginForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const email = document.getElementById("login-email").value.trim();
  const password = document.getElementById("login-password").value;
  const errorEl = document.getElementById("login-error");
  const submitBtn = document.getElementById("login-submit");

  errorEl.textContent = "";
  submitBtn.disabled = true;
  submitBtn.textContent = "Logging in…";

  try {
    await login(email, password);
    window.location.href = "dashboard.html";
  } catch (err) {
    errorEl.textContent = err.message;
    submitBtn.disabled = false;
    submitBtn.textContent = "Log in";
  }
});

registerForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const email = document.getElementById("register-email").value.trim();
  const password = document.getElementById("register-password").value;
  const errorEl = document.getElementById("register-error");
  const submitBtn = document.getElementById("register-submit");

  errorEl.textContent = "";
  submitBtn.disabled = true;
  submitBtn.textContent = "Creating account…";

  try {
    await register(email, password);
    // Registration succeeded — log the user straight in for convenience.
    await login(email, password);
    window.location.href = "dashboard.html";
  } catch (err) {
    errorEl.textContent = err.message;
    submitBtn.disabled = false;
    submitBtn.textContent = "Create account";
  }
});
