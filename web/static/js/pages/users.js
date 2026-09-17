
  let currentPage = 1;
  let currentUserRole = "";

  async function loadCurrentUserRole() {
    try {
      const response = await fetch("/auth/me", {
        credentials: "same-origin",
      });
      if (!response.ok) return;

      const result = await response.json();
      currentUserRole = result.role || "";
    } catch {
      currentUserRole = "";
    }
  }

  function showUserAlert(message, success = false) {
    const box = document.getElementById("user-alert");

    box.innerHTML = `
        <div class="alert ${success ? "alert-success" : "alert-danger"}"
             data-auto-dismiss>
            ${escapeHTML(message)}
        </div>
    `;

    setTimeout(() => {
      box.innerHTML = "";
    }, 4000);
  }

  function openUserModal() {
    document.getElementById("user-modal").classList.remove("hidden");
    document.body.style.overflow = "hidden";

    document.getElementById("user-modal-title").textContent = "Add User";

    document.getElementById("user-form").reset();

    document.getElementById("user-id").value = "";

    document.getElementById("password-group").style.display = "flex";

    document.getElementById("status-group").style.display = "none";
  }

  function closeUserModal() {
    document.getElementById("user-modal").classList.add("hidden");
    document.body.style.overflow = "";
    document.getElementById("user-form").reset();
  }

  function openPasswordModal(id) {
    document.getElementById("password-user-id").value = id;

    document.getElementById("new-password").value = "";

    document.getElementById("password-modal").classList.remove("hidden");
    document.body.style.overflow = "hidden";
  }

  function closePasswordModal() {
    document.getElementById("password-modal").classList.add("hidden");
    document.body.style.overflow = "";
    document.getElementById("password-form").reset();
  }

  document.getElementById("user-modal").addEventListener("click", (event) => {
    if (event.target === event.currentTarget) closeUserModal();
  });

  document
    .getElementById("password-modal")
    .addEventListener("click", (event) => {
      if (event.target === event.currentTarget) closePasswordModal();
    });

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    const userModal = document.getElementById("user-modal");
    const passwordModal = document.getElementById("password-modal");
    if (passwordModal && !passwordModal.classList.contains("hidden")) {
      closePasswordModal();
    } else if (userModal && !userModal.classList.contains("hidden")) {
      closeUserModal();
    }
  });

  // User-facing role terminology: the internal "Partner" role is
  // displayed as "Manager" everywhere in the UI.
  function roleDisplay(role) {
    return role === "Partner" ? "Manager" : role;
  }

  function statusBadge(status) {
    if (status === "Active") {
      return `<span class="badge badge-success">${escapeHTML(status)}</span>`;
    }

    if (status === "Pending") {
      return `<span class="badge badge-warning">${escapeHTML(status)}</span>`;
    }

    if (status === "Disabled" || status === "Locked") {
      return `<span class="badge badge-danger">${escapeHTML(status)}</span>`;
    }

    return `<span class="badge badge-muted">${escapeHTML(status)}</span>`;
  }

  function escapeHTML(value) {
    return String(value ?? "")
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;")
      .replaceAll("'", "&#039;");
  }

  async function loadUsers(page = 1) {
    currentPage = page;

    const table = document.getElementById("users-table");

    const form = document.getElementById("user-search-form");

    const params = new URLSearchParams();

    const search = form.querySelector("[name=search]").value.trim();

    const role = form.querySelector("[name=role]").value;

    if (search) {
      params.set("search", search);
    }

    if (role) {
      params.set("role", role);
    }

    params.set("page", page);
    params.set("page_size", "10");

    table.innerHTML = `
        <tr>
            <td colspan="6">
                <div class="empty">Loading users...</div>
            </td>
        </tr>
    `;

    try {
      const response = await fetch(`/users?${params.toString()}`, {
        headers: {
          Accept: "application/json",
        },
      });

      const result = await response.json();

      if (!response.ok || !result.success) {
        throw new Error(result.message || "Unable to load users");
      }

      const users = result.data || [];

      const pagination = result.pagination || {};

      document.getElementById("user-count").textContent =
        `${pagination.total_items ?? users.length} Records`;

      if (users.length === 0) {
        table.innerHTML = `
                <tr>
                    <td colspan="6">
                        <div class="empty">No users found.</div>
                    </td>
                </tr>
            `;

        renderPagination(pagination);

        return;
      }

      table.innerHTML = users
        .map((user) => {
          const canModifyTarget =
            currentUserRole &&
            (currentUserRole === "Super Admin" || user.role !== "Super Admin");
          const canDelete = currentUserRole === "Super Admin";

          return `

            <tr>

                <td>
                    <strong>${escapeHTML(user.username)}</strong>
                </td>

                <td>
                    ${escapeHTML(user.email)}
                </td>

                <td>
                    <span class="badge badge-muted">
                        ${escapeHTML(roleDisplay(user.role))}
                    </span>
                </td>

                <td>
                    ${statusBadge(user.status)}
                </td>

                <td>
                    ${
                      user.last_login_at
                        ? new Date(user.last_login_at).toLocaleString()
                        : "Never"
                    }
                </td>

                <td>

                    <div style="display:flex;gap:6px;flex-wrap:wrap;">

                        ${
                          canModifyTarget
                            ? `<button
                            class="btn btn-light"
                            type="button"
                            data-csp-action="users:edit" data-csp-id="${user.id}">
                            Edit
                        </button>`
                            : ""
                        }

                        ${
                          canModifyTarget
                            ? `<button
                            class="btn btn-light"
                            type="button"
                            data-csp-action="users:change-password" data-csp-id="${user.id}">
                            Password
                        </button>`
                            : ""
                        }

                        ${
                          canModifyTarget
                            ? `<button
                            class="btn btn-light"
                            type="button"
                            data-csp-action="users:change-status" data-csp-id="${user.id}" data-csp-status="${user.status}">
                            Status
                        </button>`
                            : ""
                        }

                        ${
                          canDelete
                            ? `<button
                            class="btn btn-danger"
                            type="button"
                            data-csp-action="users:delete" data-csp-id="${user.id}" data-csp-name="${escapeHTML(user.username)}">
                            Delete
                        </button>`
                            : ""
                        }

                    </div>

                </td>

            </tr>

        `;
        })
        .join("");

      renderPagination(pagination);
    } catch (error) {
      table.innerHTML = `
            <tr>
                <td colspan="6">
                    <div class="empty">
                        ${escapeHTML(error.message)}
                    </div>
                </td>
            </tr>
        `;
    }
  }

  function renderPagination(pagination) {
    const container = document.getElementById("users-pagination");

    const page = pagination.page || 1;

    const totalPages = pagination.total_pages || 1;

    if (totalPages <= 1) {
      container.innerHTML = "";
      return;
    }

    container.innerHTML = `

        <div style="display:flex;justify-content:center;gap:8px;align-items:center;">

            <button
                class="btn btn-light"
                ${page <= 1 ? "disabled" : ""}
                data-csp-action="users:page" data-csp-page="${page - 1}">
                Previous
            </button>

            <span>
                Page ${page} of ${totalPages}
            </span>

            <button
                class="btn btn-light"
                ${page >= totalPages ? "disabled" : ""}
                data-csp-action="users:page" data-csp-page="${page + 1}">
                Next
            </button>

        </div>

    `;
  }

  async function editUser(id) {
    try {
      const response = await fetch(`/users/${id}`, {
        headers: {
          Accept: "application/json",
        },
      });

      const result = await response.json();

      if (!response.ok || !result.success) {
        throw new Error(result.message || "Unable to load user");
      }

      const user = result.user;

      document.getElementById("user-id").value = user.id;

      document.getElementById("user-username").value = user.username;

      document.getElementById("user-email").value = user.email;

      document.getElementById("user-role").value = user.role;

      document.getElementById("user-status").value = user.status;

      document.getElementById("user-modal-title").textContent = "Edit User";

      document.getElementById("password-group").style.display = "none";

      document.getElementById("status-group").style.display = "flex";

      document.getElementById("user-modal").classList.remove("hidden");
      document.body.style.overflow = "hidden";
    } catch (error) {
      showUserAlert(error.message);
    }
  }

  document
    .getElementById("user-form")
    .addEventListener("submit", async (event) => {
      event.preventDefault();

      const id = document.getElementById("user-id").value;

      const username = document.getElementById("user-username").value.trim();

      const email = document.getElementById("user-email").value.trim();

      const role = document.getElementById("user-role").value;

      try {
        let url = "/users";

        let method = "POST";

        let body = {
          username,
          email,
          role,
        };

        if (id) {
          url = `/users/${id}`;

          method = "PUT";

          body.status = document.getElementById("user-status").value;
        } else {
          body.password = document.getElementById("user-password").value;
        }

        const response = await fetch(url, {
          method,
          headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
          },
          body: JSON.stringify(body),
        });

        const result = await response.json();

        if (!response.ok || !result.success) {
          throw new Error(result.message || "Unable to save user");
        }

        closeUserModal();

        showUserAlert(result.message || "User saved successfully", true);

        await loadUsers(currentPage);
      } catch (error) {
        showUserAlert(error.message);
      }
    });

  async function changeUserStatus(id, currentStatus) {
    const statuses = ["Active", "Disabled", "Locked", "Pending"];

    const selected = prompt(
      `Enter new status:\n\n${statuses.join("\n")}`,
      currentStatus,
    );

    if (!selected || !statuses.includes(selected)) {
      return;
    }

    try {
      const response = await fetch(`/users/${id}/status`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify({
          status: selected,
        }),
      });

      const result = await response.json();

      if (!response.ok || !result.success) {
        throw new Error(result.message || "Unable to update status");
      }

      showUserAlert(result.message || "Status updated", true);

      await loadUsers(currentPage);
    } catch (error) {
      showUserAlert(error.message);
    }
  }

  async function changeUserPassword(id) {
    openPasswordModal(id);
  }

  document
    .getElementById("password-form")
    .addEventListener("submit", async (event) => {
      event.preventDefault();

      const id = document.getElementById("password-user-id").value;

      const newPassword = document.getElementById("new-password").value;

      try {
        const response = await fetch(`/users/${id}/password`, {
          method: "PATCH",
          headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
          },
          body: JSON.stringify({
            new_password: newPassword,
          }),
        });

        const result = await response.json();

        if (!response.ok || !result.success) {
          throw new Error(result.message || "Unable to reset password");
        }

        closePasswordModal();

        showUserAlert("Password changed successfully", true);
      } catch (error) {
        showUserAlert(error.message);
      }
    });

  async function deleteUser(id, username) {
    if (
      !confirm(`Delete user "${username}"?\n\nThis action cannot be undone.`)
    ) {
      return;
    }

    try {
      const response = await fetch(`/users/${id}`, {
        method: "DELETE",
        headers: {
          Accept: "application/json",
        },
      });

      const result = await response.json();

      if (!response.ok || !result.success) {
        throw new Error(result.message || "Unable to delete user");
      }

      showUserAlert("User deleted successfully", true);

      await loadUsers(currentPage);
    } catch (error) {
      showUserAlert(error.message);
    }
  }

  document
    .getElementById("user-search-form")
    .addEventListener("submit", (event) => {
      event.preventDefault();
      loadUsers(1);
    });

  document.addEventListener("DOMContentLoaded", () => {
    loadCurrentUserRole().then(() => loadUsers(1));
  });

