"use strict";
// Permission-management UI: role selector -> grouped permission checkboxes ->
// Save writes the role's permission set back through the existing
// /permissions/roles API (server-side authorization still governs changes).

const _el = (id) => document.getElementById(id);
const roleSelect = _el("permission-role-select");
const matrix = _el("permissions-matrix");
const loading = _el("permissions-loading");
const errorState = _el("permissions-error");
const actions = _el("permissions-actions");
const saveButton = _el("permissions-save");
const resetButton = _el("permissions-reset");
const alertBox = _el("permissions-alert");

let catalog = []; // { name, description, category }
let rolePermissions = {}; // role name -> Set(permission names)
let currentRole = "";
let originalSelection = new Set();

function getCsrfToken() {
  return document.cookie
    .split("; ")
    .find((row) => row.startsWith("pwams_csrf="))
    ?.split("=")
    .slice(1)
    .join("=") || "";
}

function setAlert(message, kind) {
  alertBox.textContent = message;
  alertBox.className = `alert alert-${kind === "success" ? "success" : "danger"}`;
  alertBox.hidden = false;
}
function clearAlert() {
  alertBox.hidden = true;
  alertBox.textContent = "";
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

async function loadCatalog() {
  const result = await fetchJson("/permissions");
  catalog = result.permissions || [];
}

async function loadRoles() {
  const result = await fetchJson("/permissions/roles");
  const roles = result.roles || [];
  roleSelect.innerHTML = '<option value="">Select role</option>';
  for (const role of roles) {
    rolePermissions[role.name] = new Set(role.permissions || []);
    const option = document.createElement("option");
    option.value = role.name;
    option.textContent = role.name;
    roleSelect.appendChild(option);
  }
}

function renderMatrix() {
  const selected = rolePermissions[currentRole] || new Set();
  originalSelection = new Set(selected);

  const groups = {};
  for (const perm of catalog) {
    const category = perm.category || "other";
    if (!groups[category]) groups[category] = [];
    groups[category].push(perm);
  }

  matrix.innerHTML = "";
  const titles = {
    users: "Users", super_admin: "Super Admin", admin: "Admin",
    staff: "Staff", volunteer: "Volunteer", donor: "Donor",
    beneficiary: "Beneficiary", person: "Care Seekers", student: "Students",
    donation: "Donations", aid: "Aid", care: "Care", loan: "Loans",
    repayment: "Repayments", revenue: "Revenue", file: "Files",
    message: "Messages", notification: "Notifications", reports: "Reports",
    audit: "Audit", permissions: "Permissions", account: "Account",
    other: "Other",
  };

  for (const [category, perms] of Object.entries(groups)) {
    const section = document.createElement("div");
    section.className = "panel permission-group";
    const heading = document.createElement("div");
    heading.className = "panel-header";
    const h3 = document.createElement("h3");
    h3.textContent = titles[category] || category;
    heading.appendChild(h3);
    section.appendChild(heading);

    const body = document.createElement("div");
    body.className = "panel-body permission-checks";
    for (const perm of perms) {
      const label = document.createElement("label");
      label.className = "permission-check";
      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.value = perm.name;
      checkbox.checked = selected.has(perm.name);
      checkbox.dataset.permission = perm.name;
      checkbox.addEventListener("change", () => {
        if (checkbox.checked) selected.add(perm.name);
        else selected.delete(perm.name);
      });
      const span = document.createElement("span");
      span.textContent = perm.name;
      if (perm.description) span.title = perm.description;
      label.appendChild(checkbox);
      label.appendChild(span);
      body.appendChild(label);
    }
    section.appendChild(body);
    matrix.appendChild(section);
  }

  matrix.hidden = false;
  actions.hidden = false;
}

async function selectRole() {
  currentRole = roleSelect.value;
  clearAlert();
  if (!currentRole) {
    matrix.hidden = true;
    actions.hidden = true;
    return;
  }
  loading.hidden = false;
  errorState.hidden = true;
  matrix.hidden = true;
  actions.hidden = true;
  try {
    renderMatrix();
  } catch (err) {
    errorState.hidden = false;
  } finally {
    loading.hidden = true;
  }
}

async function savePermissions() {
  if (!currentRole) return;
  const selected = rolePermissions[currentRole] || new Set();
  const names = Array.from(selected);
  saveButton.disabled = true;
  setAlert("Saving permissions...", "success");
  try {
    await fetchJson(`/permissions/roles/${encodeURIComponent(currentRole)}`, {
      method: "PUT",
      body: JSON.stringify({ permissions: names }),
    });
    rolePermissions[currentRole] = new Set(names);
    originalSelection = new Set(names);
    setAlert("Permissions saved and audited.", "success");
  } catch (err) {
    setAlert(err.message, "danger");
  } finally {
    saveButton.disabled = false;
  }
}

function resetPermissions() {
  if (!currentRole) return;
  rolePermissions[currentRole] = new Set(originalSelection);
  renderMatrix();
  clearAlert();
}

async function init() {
  loading.hidden = false;
  try {
    await loadCatalog();
    await loadRoles();
    loading.hidden = true;
  } catch (err) {
    loading.hidden = true;
    errorState.hidden = false;
    return;
  }
  roleSelect.addEventListener("change", selectRole);
  saveButton.addEventListener("click", savePermissions);
  resetButton.addEventListener("click", resetPermissions);
}

document.addEventListener("DOMContentLoaded", init);
