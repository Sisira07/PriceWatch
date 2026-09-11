// item.js — logic for item.html.

requireAuth();
wireLogoutButton();

const params = new URLSearchParams(window.location.search);
const itemId = params.get("id");

const loadingEl = document.getElementById("loading");
const notFoundEl = document.getElementById("not-found");
const contentEl = document.getElementById("item-content");

if (!itemId) {
  loadingEl.style.display = "none";
  notFoundEl.style.display = "block";
} else {
  loadItem();
}

async function loadItem() {
  loadingEl.style.display = "block";
  contentEl.style.display = "none";
  notFoundEl.style.display = "none";

  try {
    const [item, history, notifications] = await Promise.all([
      getItem(itemId),
      getItemHistory(itemId),
      getItemNotifications(itemId),
    ]);

    renderItem(item);
    renderHistory(history || []);
    renderNotifications(notifications || []);

    loadingEl.style.display = "none";
    contentEl.style.display = "block";
  } catch (err) {
    loadingEl.style.display = "none";
    notFoundEl.style.display = "block";
    showToast(err.message, "error");
  }
}

function renderItem(item) {
  document.getElementById("item-name").textContent = item.name;

  const statusBadge = document.getElementById("item-status-badge");
  statusBadge.textContent = item.is_active ? "Active" : "Paused";
  statusBadge.className = "badge " + (item.is_active ? "badge-active" : "badge-inactive");

  const urlLink = document.getElementById("item-url-link");
  urlLink.href = item.url;
  urlLink.textContent = item.url;

  document.getElementById("item-target-price").textContent = formatMoney(item.target_price);
  document.getElementById("item-current-price").textContent = formatMoney(item.current_price);

  document.getElementById("edit-name").value = item.name;
  document.getElementById("edit-url").value = item.url;
  document.getElementById("edit-target").value = item.target_price;
  document.getElementById("edit-active").checked = item.is_active;
}

function renderHistory(history) {
  const table = document.getElementById("history-table");
  const empty = document.getElementById("history-empty");
  const body = document.getElementById("history-body");
  body.innerHTML = "";

  if (!history.length) {
    table.style.display = "none";
    empty.style.display = "block";
    return;
  }

  table.style.display = "table";
  empty.style.display = "none";

  for (const row of history) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td>${formatMoney(row.price)}</td><td>${formatDateTime(row.checked_at)}</td>`;
    body.appendChild(tr);
  }
}

function renderNotifications(notifications) {
  const list = document.getElementById("notification-list");
  const empty = document.getElementById("notifications-empty");
  list.innerHTML = "";

  if (!notifications.length) {
    empty.style.display = "block";
    return;
  }
  empty.style.display = "none";

  for (const n of notifications) {
    const div = document.createElement("div");
    div.className = "notification-item";
    div.innerHTML = `
      <div>
        <div class="message">${escapeHtml(n.message)}</div>
        <div class="meta">${formatDateTime(n.created_at)} · price ${formatMoney(n.price_at_alert)}</div>
      </div>
      <span class="badge ${n.is_read ? "badge-read" : "badge-unread"}">${n.is_read ? "Read" : "New"}</span>
    `;
    list.appendChild(div);
  }
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

// --- Edit / delete ---

document.getElementById("edit-item-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const errorEl = document.getElementById("edit-error");
  const submitBtn = document.getElementById("edit-submit");
  errorEl.textContent = "";
  submitBtn.disabled = true;
  submitBtn.textContent = "Saving…";

  try {
    await updateItem(itemId, {
      name: document.getElementById("edit-name").value.trim(),
      url: document.getElementById("edit-url").value.trim(),
      target_price: parseFloat(document.getElementById("edit-target").value),
      is_active: document.getElementById("edit-active").checked,
    });
    showToast("Changes saved.", "success");
    await loadItem();
  } catch (err) {
    errorEl.textContent = err.message;
  } finally {
    submitBtn.disabled = false;
    submitBtn.textContent = "Save changes";
  }
});

document.getElementById("delete-btn").addEventListener("click", async () => {
  if (!confirm("Delete this watched item? This can't be undone.")) return;

  try {
    await deleteItem(itemId);
    window.location.href = "dashboard.html";
  } catch (err) {
    showToast(err.message, "error");
  }
});

// --- Simulate tick ---

document.getElementById("simulate-btn").addEventListener("click", async () => {
  const btn = document.getElementById("simulate-btn");
  btn.disabled = true;
  btn.textContent = "Checking prices…";

  try {
    const result = await simulateTick();
    showToast(
      `Checked ${result.items_checked} item(s), ${result.notifications_created} new notification(s).`,
      "success"
    );
    await loadItem();
  } catch (err) {
    showToast(err.message, "error");
  } finally {
    btn.disabled = false;
    btn.textContent = "Simulate price check now";
  }
});
