
  async function loadAuditLogs() {
    const container = document.getElementById("audit-log-container");

    try {
      const response = await fetch("/audit-logs", {
        method: "GET",
        credentials: "same-origin",
      });

      if (!response.ok) {
        throw new Error("Failed to load audit logs");
      }

      const result = await response.json();

      const logs = Array.isArray(result)
        ? result
        : result.data || result.logs || result.audit_logs || [];

      if (!logs.length) {
        container.innerHTML = '<div class="empty">No audit logs found.</div>';
        return;
      }

      container.innerHTML = `
      <div style="overflow-x:auto;">
        <table class="table">
          <thead>
            <tr>
              <th>Action</th>
              <th>Entity</th>
              <th>Details</th>
              <th>Created At</th>
            </tr>
          </thead>
          <tbody>
            ${logs
              .map(
                (log) => `
              <tr>
                <td>${escapeHtml(log.action || "-")}</td>
                <td>${escapeHtml(log.entity || "-")}</td>
                <td>${escapeHtml(log.details || "-")}</td>
                <td>${formatDate(log.created_at)}</td>
              </tr>
            `,
              )
              .join("")}
          </tbody>
        </table>
      </div>
    `;
    } catch (error) {
      console.error("Audit log loading error:", error);

      container.innerHTML =
        '<div class="empty">Unable to load audit logs.</div>';
    }
  }

  function escapeHtml(value) {
    return String(value)
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;")
      .replaceAll("'", "&#039;");
  }

  function formatDate(value) {
    if (!value) return "-";

    const date = new Date(value);

    if (Number.isNaN(date.getTime())) {
      return value;
    }

    return date.toLocaleString();
  }

  document.addEventListener("DOMContentLoaded", loadAuditLogs);

