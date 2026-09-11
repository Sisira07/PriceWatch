// dashboard.js — logic for dashboard.html.

requireAuth();
wireLogoutButton();

const itemGrid = document.getElementById("item-grid");
const itemsLoading = document.getElementById("items-loading");
const itemsEmpty = document.getElementById("items-empty");

async function loadItems() {
  itemsLoading.style.display = "block";
  itemsEmpty.style.display = "none";
  itemGrid.innerHTML = "";

  try {
    const items = await listItems();
    itemsLoading.style.display = "none";

    if (!items || items.length === 0) {
      itemsEmpty.style.display = "block";
      return;
    }

    for (const item of items) {
      itemGrid.appendChild(renderItemCard(item));
    }
  } catch (err) {
    itemsLoading.style.display = "none";
    showToast(err.message, "error");
  }
}

function renderItemCard(item) {
  const card = document.createElement("div");
  card.className = "item-card";

  const belowTarget = item.current_price !== null && item.current_price !== undefined
    && item.current_price <= item.target_price;

  card.innerHTML = `
    <h3>${escapeHtml(item.name)}</h3>
    <div class="url">${escapeHtml(item.url)}</div>
    <div class="price-row">
      <span class="price-target">Target: ${formatMoney(item.target_price)}</span>
      <span class="price-current ${belowTarget ? "below-target" : "above-target"}">
        ${formatMoney(item.current_price)}
      </span>
    </div>
    <span class="badge ${item.is_active ? "badge-active" : "badge-inactive"}">
      ${item.is_active ? "Active" : "Paused"}
    </span>
    <div class="item-card-actions">
      <a class="btn btn-secondary btn-sm" href="item.html?id=${encodeURIComponent(item.id)}">View details</a>
    </div>
  `;
  return card;
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

// --- Create item ---

document.getElementById("create-item-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = document.getElementById("item-name").value.trim();
  const url = document.getElementById("item-url").value.trim();
  const target_price = parseFloat(document.getElementById("item-target").value);
  const errorEl = document.getElementById("create-item-error");
  const submitBtn = document.getElementById("create-item-submit");

  errorEl.textContent = "";
  submitBtn.disabled = true;
  submitBtn.textContent = "Adding…";

  try {
    await createItem({ name, url, target_price });
    document.getElementById("create-item-form").reset();
    showToast("Item added — the worker will check its price soon.", "success");
    await loadItems();
  } catch (err) {
    errorEl.textContent = err.message;
  } finally {
    submitBtn.disabled = false;
    submitBtn.textContent = "Add item";
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
    await loadItems();
  } catch (err) {
    showToast(err.message, "error");
  } finally {
    btn.disabled = false;
    btn.textContent = "Simulate price check now";
  }
});

loadItems();
