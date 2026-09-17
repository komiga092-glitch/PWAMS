"use strict";
// Role-scoped account management UI (Step 2: admins / partners / staff /
// volunteers). One shared module driven by window.PWAMS_MANAGED and the
// server-rendered permission set (window.PWAMS_PERMISSIONS). Button
// visibility here is UX only — the backend enforces every action.

const cfg = window.PWAMS_MANAGED || {};
const permissions = window.PWAMS_PERMISSIONS || {};

const _el = (id) => document.getElementById(id);

const hasPermission = (name) => Boolean(permissions[name]);

// Mirrors handlers.requiredTargetPermission (user_handler.go): Admin targets
// require admin.*, Partner targets partner.*. Deleting an Admin account is
// permission-split into a request/approve/reject workflow — the direct
// delete action (and workflow approval) uses admin.delete.approve. The
// backend remains the enforcement point — these checks only drive button
// visibility.
function requiredTargetPermission(action) {
  if (cfg.role === "Admin") {
    if (action === "delete") return "admin.delete.approve";
    return `admin.${action}`;
  }
  if (cfg.role === "Partner") return `partner.${action}`;
  return "";
}

// The module's own permission for an action (staff.edit, volunteer.view, ...).
function modulePermission(action) {
  return `${String(cfg.role || "").toLowerCase()}.${action}`;
}

// Effective permission required for an action on this module's accounts,
// mirroring route gates + in-handler checks:
//   - admin/partner targets: the role-specific permission
//   - staff/volunteer: the module permission (status changes use .edit)
function canDo(action) {
  const extra = requiredTargetPermission(action);
  if (extra) return hasPermission(extra);
  if (action === "activate" || action === "deactivate") {
    return hasPermission(modulePermission("edit"));
  }
  return hasPermission(modulePermission(action));
}

function getCsrfToken() {
  return (
    document.cookie
      .split("; ")
      .find((row) => row.startsWith("pwams_csrf="))
      ?.split("=")
      .slice(1)
      .join("=") || ""
  );
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

let alertTimer = null;
function setAlert(message, kind) {
  const box = _el("managed-alert");
  if (!box) return;
  box.textContent = message;
  box.className = `alert alert-${kind === "success" ? "success" : "danger"}`;
  box.hidden = false;
  clearTimeout(alertTimer);
  alertTimer = setTimeout(() => {
    box.hidden = true;
  }, 6000);
}

async function fetchJson(url, options = {}) {
  const headers = {
    Accept: "application/json",
    "X-CSRF-Token": getCsrfToken(),
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(options.headers || {}),
  };
  const response = await fetch(url, { credentials: "same-origin", ...options, headers });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.message || `Request failed (${response.status})`);
  }
  return payload;
}

function statusBadge(status) {
  const value = escapeHTML(status);
  if (status === "Active") return `<span class="badge badge-success">${value}</span>`;
  if (status === "Disabled" || status === "Locked")
    return `<span class="badge badge-danger">${value}</span>`;
  return `<span class="badge badge-muted">${value}</span>`;
}

function formatLogin(value) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString();
}

// ---------------------------------------------------------------------------
// List rendering (search / status filter / pagination — all server-side)
// ---------------------------------------------------------------------------

let currentPage = 1;

async function loadAccounts(page = 1) {
  currentPage = page;
  const table = _el("managed-table");
  const form = _el("managed-search-form");

  const params = new URLSearchParams();
  const search = form?.querySelector("[name=search]")?.value.trim();
  const status = form?.querySelector("[name=status]")?.value;
  if (search) params.set("search", search);
  if (status) params.set("status", status);
  params.set("page", String(page));
  params.set("page_size", "10");

  table.innerHTML = `<tr><td colspan="6"><div class="empty">Loading accounts...</div></td></tr>`;

  try {
    const result = await fetchJson(`${cfg.apiBase}?${params.toString()}`);
    const accounts = result.data || [];
    const pagination = result.pagination || {};

    _el("managed-count").textContent = `${pagination.total_items ?? accounts.length} Records`;

    if (accounts.length === 0) {
      table.innerHTML = `<tr><td colspan="6"><div class="empty">No ${escapeHTML(cfg.label || "")} accounts found.</div></td></tr>`;
      renderPagination(pagination);
      return;
    }

    table.innerHTML = accounts.map((account) => rowHTML(account)).join("");
    renderPagination(pagination);
  } catch (err) {
    table.innerHTML = `<tr><td colspan="6"><div class="empty">${escapeHTML(err.message)}</div></td></tr>`;
  }
}

function rowActions(account) {
  const actions = [];
  actions.push(
    `<button class="btn btn-light" type="button" data-view="${account.id}">View</button>`
  );
  if (canDo("edit")) {
    actions.push(
      `<button class="btn btn-primary" type="button" data-edit="${account.id}">Edit</button>`
    );
  }
  if (account.status === "Active" && canDo("deactivate")) {
    actions.push(
      `<button class="btn btn-light" type="button" data-status="${account.id}" data-next="Disabled">Deactivate</button>`
    );
  } else if (account.status !== "Active" && canDo("activate")) {
    actions.push(
      `<button class="btn btn-light" type="button" data-status="${account.id}" data-next="Active">Activate</button>`
    );
  }
  if (cfg.deletable && (cfg.role === "Admin" ? canDo("delete.request") : canDo("delete"))) {
    if (cfg.role === "Admin") {
      // Admin module: never offer a direct destructive delete to a normal
      // Admin. The supported removal path is the supervised
      // request/approve workflow (admin.delete.request), and self-delete
      // controls are hidden. The backend independently rejects direct Admin
      // deletes (Super Admin role only) and self-requests.
      if (account.id !== cfg.actorId) {
        actions.push(
          `<button class="btn btn-danger" type="button" data-request-delete="${account.id}" data-name="${escapeHTML(account.full_name || account.username)}">Request Delete</button>`
        );
      }
    } else {
      actions.push(
        `<button class="btn btn-danger" type="button" data-delete="${account.id}" data-name="${escapeHTML(account.full_name || account.username)}">Delete</button>`
      );
    }
  }
  return actions.length ? `<div class="table-actions">${actions.join("")}</div>` : "";
}

function rowHTML(account) {
  return `<tr>
    <td>${escapeHTML(account.full_name || "—")}</td>
    <td>${escapeHTML(account.username)}</td>
    <td>${escapeHTML(account.email)}</td>
    <td>${statusBadge(account.status)}</td>
    <td>${formatLogin(account.last_login_at)}</td>
    <td>${rowActions(account)}</td>
  </tr>`;
}

function renderPagination(pagination) {
  const container = _el("managed-pagination");
  if (!container) return;

  const totalPages = pagination.total_pages || 0;
  if (totalPages <= 1) {
    container.innerHTML = "";
    return;
  }

  const buttons = [];
  for (let page = 1; page <= totalPages; page += 1) {
    const active = page === currentPage ? "btn-primary" : "btn-light";
    buttons.push(
      `<button class="btn ${active}" type="button" data-page="${page}">${page}</button>`
    );
  }
  container.innerHTML = `<div class="table-actions">${buttons.join("")}</div>`;
}

// ---------------------------------------------------------------------------
// Modals
// ---------------------------------------------------------------------------

const accountCache = new Map();

function openModal(id) {
  _el(id).classList.remove("hidden");
  document.body.style.overflow = "hidden";
}

function closeModal(id) {
  _el(id).classList.add("hidden");
  document.body.style.overflow = "";
}

async function ensureAccount(id) {
  if (accountCache.has(id)) return accountCache.get(id);
  const result = await fetchJson(`${cfg.apiBase}/${id}`);
  const account = result.user;
  accountCache.set(id, account);
  return account;
}

function openCreateModal() {
  const form = _el("managed-form");
  form.reset();
  _el("managed-id").value = "";
  _el("managed-modal-title").textContent = `Add ${cfg.label}`;
  _el("managed-modal-subtitle").textContent = `Create a ${cfg.label} account.`;
  _el("managed-username-group").style.display = "";
  _el("managed-password-setup-group").style.display = "";
  _el("managed-confirm-password-group").style.display = "";
  _el("managed-temp-password").required = true;
  _el("managed-confirm-password").required = true;
  _el("managed-status-group").style.display = "none";
  _el("managed-status").required = false;
  openModal("managed-modal");
}

async function openEditModal(id) {
  try {
    const account = await ensureAccount(id);
    const form = _el("managed-form");
    form.reset();
    _el("managed-id").value = account.id;
    _el("managed-full-name").value = account.full_name || "";
    _el("managed-email").value = account.email || "";
    _el("managed-username").value = account.username || "";
    _el("managed-phone").value = account.phone || "";
    _el("managed-language").value = account.language || "";
    _el("managed-status").value = account.status || "Active";
    _el("managed-modal-title").textContent = `Edit ${cfg.label}`;
    _el("managed-modal-subtitle").textContent = `Update the ${cfg.label} account.`;
    _el("managed-username-group").style.display = "";
    _el("managed-password-setup-group").style.display = "none";
    _el("managed-confirm-password-group").style.display = "none";
    _el("managed-temp-password").required = false;
    _el("managed-confirm-password").required = false;
    _el("managed-status-group").style.display = "";
    _el("managed-status").required = true;
    openModal("managed-modal");
  } catch (err) {
    setAlert(err.message, "danger");
  }
}

async function openViewModal(id) {
  try {
    const account = await ensureAccount(id);
    _el("managed-view-full-name").textContent = account.full_name || "—";
    _el("managed-view-username").textContent = account.username || "—";
    _el("managed-view-email").textContent = account.email || "—";
    _el("managed-view-phone").textContent = account.phone || "—";
    _el("managed-view-language").textContent = account.language || "—";
    _el("managed-view-role").textContent = account.role || cfg.role;
    _el("managed-view-status").innerHTML = statusBadge(account.status);
    _el("managed-view-last-login").textContent = formatLogin(account.last_login_at);
    openModal("managed-view-modal");
  } catch (err) {
    setAlert(err.message, "danger");
  }
}

let pendingDeleteId = null;

function openDeleteModal(id, name) {
  pendingDeleteId = id;
  const namePart = name ? ` (${name})` : "";
  const message = t("users.js.delete_confirm_managed")
    .replace("{label}", cfg.label)
    .replace("{name}", namePart);
  _el("managed-delete-message").textContent = message;
  openModal("managed-delete-modal");
}

// ---------------------------------------------------------------------------
// Mutations
// ---------------------------------------------------------------------------

async function submitForm(event) {
  event.preventDefault();

  const id = _el("managed-id").value;
  const payload = {
    full_name: _el("managed-full-name").value.trim(),
    email: _el("managed-email").value.trim(),
    username: _el("managed-username").value.trim(),
    phone: _el("managed-phone").value.trim(),
    language: _el("managed-language").value,
  };

  try {
    if (id) {
      payload.status = _el("managed-status").value;
      await fetchJson(`${cfg.apiBase}/${id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      setAlert(`${cfg.label} updated successfully.`, "success");
    } else {
      const password = _el("managed-temp-password").value;
      const confirmPassword = _el("managed-confirm-password").value;
      if (password !== confirmPassword) {
        setAlert(t("users.js.password_mismatch"), "danger");
        return;
      }
      payload.temporary_password = password;
      await fetchJson(cfg.apiBase, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      setAlert(`${cfg.label} created Active — they can log in immediately.`, "success");
    }

    closeModal("managed-modal");
    accountCache.clear();
    await loadAccounts(currentPage);
  } catch (err) {
    setAlert(err.message, "danger");
  }
}

async function changeStatus(id, nextStatus) {
  try {
    await fetchJson(`${cfg.apiBase}/${id}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status: nextStatus }),
    });
    setAlert(
      nextStatus === "Active"
        ? `${cfg.label} activated successfully.`
        : `${cfg.label} deactivated successfully.`,
      "success"
    );
    accountCache.clear();
    await loadAccounts(currentPage);
  } catch (err) {
    setAlert(err.message, "danger");
  }
}

async function confirmDelete() {
  if (!pendingDeleteId) return;
  const id = pendingDeleteId;
  pendingDeleteId = null;
  try {
    await fetchJson(`${cfg.apiBase}/${id}`, { method: "DELETE" });
    setAlert(`${cfg.label} deleted successfully.`, "success");
    closeModal("managed-delete-modal");
    accountCache.clear();
    await loadAccounts(currentPage);
  } catch (err) {
    closeModal("managed-delete-modal");
    setAlert(err.message, "danger");
  }
}

// ---------------------------------------------------------------------------
// Admin deletion request workflow (Part 4 / Part 7): request / approve / reject
// ---------------------------------------------------------------------------

// Loads and renders the pending Admin deletion queue (Admin module only). The
// list endpoint is gated by admin.view; Approve/Reject buttons are rendered
// only for requests raised by a DIFFERENT Admin that target a DIFFERENT
// account. UI hiding is UX only — the backend enforces the four-eyes rule.
async function loadDeletionQueue() {
  const table = _el("deletion-queue-table");
  const count = _el("deletion-queue-count");
  if (!table || !count || cfg.role !== "Admin") return;

  count.textContent = "0 Pending";
  table.innerHTML = `<tr><td colspan="6"><div class="empty">Loading pending requests...</div></td></tr>`;

  try {
    const result = await fetchJson(`${cfg.apiBase}/deletion-requests`);
    const requests = result.requests || [];
    count.textContent = `${requests.length} Pending`;

    if (requests.length === 0) {
      table.innerHTML = `<tr><td colspan="6"><div class="empty">No pending Admin deletion requests.</div></td></tr>`;
      return;
    }

    table.innerHTML = requests.map(deletionQueueRow).join("");
  } catch (err) {
    table.innerHTML = `<tr><td colspan="6"><div class="empty">${escapeHTML(err.message)}</div></td></tr>`;
  }
}

function deletionQueueRow(request) {
  const target = request.target_user || {};
  const requester = request.requester || {};
  const targetName = escapeHTML(target.full_name || target.username || request.target_user_id);
  const requesterName = escapeHTML(requester.full_name || requester.username || request.requester_id);
  const reason = escapeHTML(request.reason || "—");
  const at = request.requested_at || request.created_at;

  let actions = "";
  if (request.requester_id !== cfg.actorId && request.target_user_id !== cfg.actorId && hasPermission("admin.delete.approve")) {
    actions = `<div class="table-actions">
      <button class="btn btn-primary" type="button" data-approve-deletion="${request.id}">Approve</button>
      <button class="btn btn-light" type="button" data-reject-deletion="${request.id}">Reject</button>
    </div>`;
  }

  return `<tr>
    <td>${targetName}</td>
    <td>${requesterName}</td>
    <td>${reason}</td>
    <td>${formatLogin(at)}</td>
    <td>${statusBadge(request.status)}</td>
    <td>${actions || '<span class="badge badge-muted">Awaiting another Admin</span>'}</td>
  </tr>`;
}

function openRequestDeleteModal(id, name) {
  _el("managed-request-delete-id").value = id;
  _el("managed-request-delete-name").value = name || "";
  _el("managed-request-delete-target").textContent =
    `Request the deactivation of the Admin account: ${name || id}`;
  _el("managed-request-delete-reason").value = "";
  _el("managed-request-delete-modal").classList.remove("hidden");
}

function closeRequestDeleteModal() {
  const modal = _el("managed-request-delete-modal");
  if (modal) modal.classList.add("hidden");
}

async function submitRequestDelete(event) {
  event.preventDefault();
  const id = _el("managed-request-delete-id").value;
  const reason = _el("managed-request-delete-reason").value.trim();
  try {
    const payload = await fetchJson(`${cfg.apiBase}/deletion-requests`, {
      method: "POST",
      body: JSON.stringify({ target_user_id: id, reason }),
    });
    setAlert(payload.message || "Deletion request submitted for approval.", "success");
    closeRequestDeleteModal();
    await Promise.all([loadAccounts(currentPage), loadDeletionQueue()]);
  } catch (err) {
    setAlert(err.message, "danger");
  }
}

async function approveDeletionRequest(id) {
  if (!confirm("Approve this Admin deletion request? The target Admin account will be deactivated.")) return;
  try {
    const payload = await fetchJson(`${cfg.apiBase}/deletion-requests/${id}/approve`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    setAlert(payload.message || "Deletion request approved; account deactivated.", "success");
    await Promise.all([loadAccounts(currentPage), loadDeletionQueue()]);
  } catch (err) {
    setAlert(err.message, "danger");
  }
}

async function rejectDeletionRequest(id) {
  const note = window.prompt("Rejection note (optional):");
  if (note === null) return;
  try {
    const payload = await fetchJson(`${cfg.apiBase}/deletion-requests/${id}/reject`, {
      method: "POST",
      body: JSON.stringify({ response: note || "" }),
    });
    setAlert(payload.message || "Deletion request rejected; account remains active.", "success");
    await Promise.all([loadAccounts(currentPage), loadDeletionQueue()]);
  } catch (err) {
    setAlert(err.message, "danger");
  }
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------

function wireTableEvents() {
  _el("managed-table").addEventListener("click", (event) => {
    const button = event.target.closest("button");
    if (!button) return;

    if (button.dataset.view) openViewModal(button.dataset.view);
    else if (button.dataset.edit) openEditModal(button.dataset.edit);
    else if (button.dataset.status) changeStatus(button.dataset.status, button.dataset.next);
    else if (button.dataset.delete) openDeleteModal(button.dataset.delete, button.dataset.name);
    else if (button.dataset.requestDelete) openRequestDeleteModal(button.dataset.requestDelete, button.dataset.name);
  });

  _el("managed-pagination").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-page]");
    if (button) loadAccounts(Number(button.dataset.page));
  });
}

// wireDeletionQueueEvents binds the pending deletion queue's Approve/Reject
// buttons. The buttons are only rendered for requests the viewer may resolve
// (raised by a different Admin, target is a different account); the backend
// enforces the same four-eyes rule independently.
function wireDeletionQueueEvents() {
  const table = _el("deletion-queue-table");
  if (!table) return;

  table.addEventListener("click", (event) => {
    const button = event.target.closest("button");
    if (!button) return;

    if (button.dataset.approveDeletion) approveDeletionRequest(button.dataset.approveDeletion);
    else if (button.dataset.rejectDeletion) rejectDeletionRequest(button.dataset.rejectDeletion);
  });
}

function wireGlobalEvents() {
  const searchForm = _el("managed-search-form");
  if (searchForm) {
    searchForm.addEventListener("submit", (event) => {
      event.preventDefault();
      loadAccounts(1);
    });
    const searchInput = searchForm.querySelector("[name=search]");
    let debounce;
    searchInput.addEventListener("input", () => {
      clearTimeout(debounce);
      debounce = setTimeout(() => loadAccounts(1), 300);
    });
  }

  document.querySelectorAll("[data-managed-action]").forEach((button) => {
    button.addEventListener("click", () => {
      switch (button.dataset.managedAction) {
        case "open-create":
          if (canDo("create")) openCreateModal();
          else setAlert("You do not have permission to create accounts.", "danger");
          break;
        case "close-modal":
          closeModal("managed-modal");
          break;
        case "close-view":
          closeModal("managed-view-modal");
          break;
        case "close-delete":
          closeModal("managed-delete-modal");
          break;
        case "close-request-delete":
          closeRequestDeleteModal();
          break;
      }
    });
  });

  _el("managed-delete-confirm").addEventListener("click", confirmDelete);
  _el("managed-form").addEventListener("submit", submitForm);
  if (_el("managed-request-delete-form")) {
    _el("managed-request-delete-form").addEventListener("submit", submitRequestDelete);
  }

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    closeModal("managed-modal");
    closeModal("managed-view-modal");
    closeModal("managed-delete-modal");
    closeRequestDeleteModal();
  });
}

async function init() {
  if (!cfg.apiBase) return;
  wireTableEvents();
  wireDeletionQueueEvents();
  wireGlobalEvents();
  await Promise.all([loadAccounts(1), loadDeletionQueue()]);
}

document.addEventListener("DOMContentLoaded", init);
