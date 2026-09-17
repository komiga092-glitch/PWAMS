
    let currentPage = 1;
    let pageSize = 20;
    let totalPages = 1;
    let editingId = null;

    // Initialize on page load
    document.addEventListener("DOMContentLoaded", () => {
      loadCareRecords(1);
      loadCareFormOptions();
      document.getElementById("careTypeFilter").addEventListener("change", () => {
        loadCareRecords(1);
      });
    });

    async function loadCareFormOptions() {
      const personSelect = document.getElementById("carePersonID");
      const aidSelect = document.getElementById("careAidRequestID");

      try {
        const [peopleResponse, aidResponse] = await Promise.all([
          fetch("/persons?page_size=100", { credentials: "same-origin" }),
          fetch("/aid-requests?page_size=100", { credentials: "same-origin" }),
        ]);
        const people = peopleResponse.ok ? (await peopleResponse.json()).data || [] : [];
        const aidRequests = aidResponse.ok ? (await aidResponse.json()).data || [] : [];

        people.forEach((person) => {
          const option = document.createElement("option");
          option.value = person.id;
          option.textContent = `${person.full_name} (${person.nic_passport})`;
          personSelect.appendChild(option);
        });
        aidRequests.forEach((request) => {
          const option = document.createElement("option");
          option.value = request.id;
          option.textContent = `${request.title} (${request.status})`;
          aidSelect.appendChild(option);
        });
      } catch (error) {
        console.error("Care form options loading error:", error);
      }
    }

    async function loadCareRecords(page = 1) {
      const loadingSpinner = document.getElementById("loadingSpinner");
      const careTable = document.getElementById("careTable");
      const emptyState = document.getElementById("emptyState");
      const paginationContainer = document.getElementById("paginationContainer");

      loadingSpinner.style.display = "block";
      careTable.style.display = "none";
      emptyState.style.display = "none";
      paginationContainer.style.display = "none";

      try {
        if (!navigator.onLine && window.pwamsOfflineReadStore) {
          const localRecords = await window.pwamsOfflineReadStore("care_provided");
          const records = localRecords.filter((record) => !record.is_deleted);
          loadingSpinner.style.display = "none";
          if (records.length === 0) {
            emptyState.textContent = "No offline data available.";
            emptyState.style.display = "block";
            return;
          }
          const tbody = document.getElementById("careTableBody");
          tbody.innerHTML = records.map((record) => `
            <tr>
              <td>${escapeHtml(record.person_id || "-")}</td>
              <td>${escapeHtml(record.care_type || "-")}</td>
              <td>${escapeHtml(record.description || "-")}</td>
              <td>${formatCurrency(record.amount)}</td>
              <td>${formatDate(record.provided_at)}</td>
              <td>${escapeHtml(record.provided_by || "-")}</td>
              <td>${escapeHtml(record.status || "pending")}</td>
              <td></td>
            </tr>`).join("");
          document.getElementById("recordCount").textContent = `${records.length} Records`;
          careTable.style.display = "table";
          return;
        }

        const response = await fetch(
          `/care-provided?page=${page}&page_size=${pageSize}`,
          {
            method: "GET",
            credentials: "same-origin",
            headers: {
              "Content-Type": "application/json",
            },
          },
        );

        if (!response.ok) {
          throw new Error("Failed to load care records");
        }

        const result = await response.json();
        const records = result.data || [];
        const pagination = result.pagination || {};

        currentPage = pagination.page || 1;
        totalPages = pagination.total_pages || 1;

        if (records.length === 0) {
          loadingSpinner.style.display = "none";
          emptyState.style.display = "block";
          return;
        }

        // Populate table
        const tbody = document.getElementById("careTableBody");
        tbody.innerHTML = records
          .map(
            (record) => `
        <tr>
          <td>${escapeHtml(record.person_id || "-")}</td>
          <td>${escapeHtml(record.care_type || "-")}</td>
          <td>${escapeHtml(record.description || "-")}</td>
          <td>${formatCurrency(record.amount)}</td>
          <td>${formatDate(record.provided_at)}</td>
          <td>${escapeHtml(record.provided_by || "-")}</td>
          <td><span class="badge badge-${record.status?.toLowerCase() || "pending"}">${escapeHtml(record.status || "pending")}</span></td>
          <td class="actions">
            <button class="btn btn-small" data-csp-action="care:view" data-csp-id="${record.id}">View</button>
            <button class="btn btn-small" data-roles="Super Admin,Admin,Staff" data-csp-action="care:edit" data-csp-id="${record.id}">Edit</button>
            <button class="btn btn-small btn-danger" data-roles="Super Admin,Admin" data-csp-action="care:delete" data-csp-id="${record.id}">Delete</button>
          </td>
        </tr>
      `,
          )
          .join("");

        // Update record count
        document.getElementById("recordCount").textContent =
          `${pagination.total_items || 0} Records`;

        // Show table
        loadingSpinner.style.display = "none";
        careTable.style.display = "table";

        // Show pagination if needed
        if (totalPages > 1) {
          paginationContainer.style.display = "flex";
          document.getElementById("pageInfo").textContent =
            `Page ${currentPage} of ${totalPages}`;
          document.getElementById("prevBtn").disabled = currentPage === 1;
          document.getElementById("nextBtn").disabled =
            currentPage === totalPages;
        }
      } catch (error) {
        console.error("Care record loading error:", error);
        loadingSpinner.style.display = "none";
        emptyState.textContent = "Unable to load care records.";
        emptyState.style.display = "block";
      }
    }

    async function viewCareRecord(id) {
      try {
        const response = await fetch(`/care-provided/${id}`, {
          method: "GET",
          credentials: "same-origin",
        });

        if (!response.ok) {
          throw new Error("Failed to load care record");
        }

        const result = await response.json();
        const record = result.data;

        // Populate form with read-only data
        alert(`
  Care Record Details:
  Person ID: ${record.person_id}
  Care Type: ${record.care_type}
  Amount: ${record.amount}
  Date: ${formatDate(record.provided_at)}
  Status: ${record.status}
  Description: ${record.description}
      `);
      } catch (error) {
        console.error("Error viewing care record:", error);
        alert("Unable to load care record details.");
      }
    }

    async function editCareRecord(id) {
      try {
        const response = await fetch(`/care-provided/${id}`, {
          method: "GET",
          credentials: "same-origin",
        });

        if (!response.ok) {
          throw new Error("Failed to load care record");
        }

        const result = await response.json();
        const record = result.data;

        // Set editing mode
        editingId = id;
        document.getElementById("modalTitle").textContent = "Edit Care Record";
        document.getElementById("modalSubtitle").textContent =
          "Update care assistance details.";

        // Populate form fields
        document.getElementById("careAidRequestID").value =
          record.aid_request_id || "";
        document.getElementById("carePersonID").value = record.person_id || "";
        document.getElementById("careType").value = record.care_type || "";
        document.getElementById("providedDate").value =
          record.provided_at?.split("T")[0] || "";
        document.getElementById("providedBy").value = record.provided_by || "";
        document.getElementById("careAmount").value = record.amount || "";
        document.getElementById("careDescription").value =
          record.description || "";

        openCareModal();
      } catch (error) {
        console.error("Error editing care record:", error);
        alert("Unable to load care record for editing.");
      }
    }

    async function handleCareFormSubmit(event) {
      event.preventDefault();

      const submitBtn = document.getElementById("submitBtn");
      const formMessage = document.getElementById("formMessage");
      submitBtn.disabled = true;
      formMessage.style.display = "none";

      try {
        const formData = new FormData(document.getElementById("careForm"));
        const requestBody = {
          aid_request_id: formData.get("aid_request_id"),
          person_id: formData.get("person_id"),
          care_type: formData.get("care_type"),
          provided_by: formData.get("provided_by"),
          provided_at: formData.get("provided_at"),
          amount: parseFloat(formData.get("amount")),
          description: formData.get("description"),
        };

        if (!navigator.onLine) {
          const { savePendingMutation, offlineSuccessMessage } = await import("/static/js/offline/mutations.js");
          const operation = editingId ? "UPDATE" : "CREATE";
          await savePendingMutation("care_provided", operation, requestBody, editingId || undefined);
          formMessage.className = "form-message success";
          formMessage.textContent = offlineSuccessMessage("care_provided", operation);
          formMessage.style.display = "block";
          setTimeout(() => {
            closeCareModal();
            document.getElementById("careForm").reset();
            editingId = null;
            loadCareRecords(1);
          }, 1500);
          return;
        }

        let response;
        if (editingId) {
          // Update existing record
          response = await fetch(`/care-provided/${editingId}`, {
            method: "PUT",
            credentials: "same-origin",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify(requestBody),
          });
        } else {
          // Create new record
          response = await fetch("/care-provided", {
            method: "POST",
            credentials: "same-origin",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify(requestBody),
          });
        }

        const result = await response.json();

        if (!response.ok) {
          throw new Error(result.message || "Failed to save care record");
        }

        formMessage.className = "form-message success";
        formMessage.textContent =
          result.message || "Care record saved successfully";
        formMessage.style.display = "block";

        setTimeout(() => {
          closeCareModal();
          editingId = null;
          document.getElementById("careForm").reset();
          loadCareRecords(1);
        }, 1500);
      } catch (error) {
        console.error("Error saving care record:", error);
        formMessage.className = "form-message error";
        formMessage.textContent = error.message || "Failed to save care record";
        formMessage.style.display = "block";
      } finally {
        submitBtn.disabled = false;
      }
    }

    async function deleteCareRecord(id) {
      if (!confirm("Are you sure you want to delete this care record?")) {
        return;
      }

      try {
        const response = await fetch(`/care-provided/${id}`, {
          method: "DELETE",
          credentials: "same-origin",
        });

        const result = await response.json();

        if (!response.ok) {
          throw new Error(result.message || "Failed to delete care record");
        }

        alert(result.message || "Care record deleted successfully");
        loadCareRecords(currentPage);
      } catch (error) {
        console.error("Error deleting care record:", error);
        alert("Failed to delete care record: " + error.message);
      }
    }

    function openCareModal() {
      if (editingId === null) {
        document.getElementById("modalTitle").textContent = "Add Care Record";
        document.getElementById("modalSubtitle").textContent =
          "Record welfare assistance provided.";
        document.getElementById("careForm").reset();
        document.getElementById("submitBtn").textContent = "Save Care Record";
      } else {
        document.getElementById("submitBtn").textContent = "Update Care Record";
      }
      document.getElementById("formMessage").style.display = "none";
      const modal = document.getElementById("careModal");
      if (modal) {
        modal.classList.remove("hidden");
        document.body.style.overflow = "hidden";
      }
    }

    function closeCareModal() {
      const modal = document.getElementById("careModal");
      if (modal) {
        modal.classList.add("hidden");
      }
      document.body.style.overflow = "";
      document.getElementById("careForm").reset();
      editingId = null;
    }

    document.addEventListener("keydown", (event) => {
      const modal = document.getElementById("careModal");
      if (event.key === "Escape" && modal && !modal.classList.contains("hidden")) {
        closeCareModal();
      }
    });

    function previousPage() {
      if (currentPage > 1) {
        loadCareRecords(currentPage - 1);
      }
    }

    function nextPage() {
      if (currentPage < totalPages) {
        loadCareRecords(currentPage + 1);
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
      return date.toLocaleDateString();
    }

    function formatCurrency(value) {
      if (!value) return "-";
      return new Intl.NumberFormat("en-US", {
        style: "currency",
        currency: "USD",
      }).format(value);
    }

