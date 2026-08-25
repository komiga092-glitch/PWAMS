const notificationList = document.getElementById("notification-list");
const loadingState = document.getElementById("notifications-loading");
const emptyState = document.getElementById("notifications-empty");
const errorState = document.getElementById("notifications-error");
const viewModal = document.getElementById("notification-view-modal");
const createModal = document.getElementById("create-notification-modal");
const createForm = document.getElementById("create-notification-form");
const createFeedback = document.getElementById("create-notification-feedback");
const createSubmit = document.getElementById("create-notification-submit");
const recipientSelect = document.getElementById("notification-recipient");
let selectedNotificationID = "";
async function request(url, options) {
    const response = await fetch(url, {
        credentials: "same-origin",
        headers: { Accept: "application/json", ...options?.headers },
        ...options,
    });
    const result = (await response.json().catch(() => ({})));
    if (!response.ok)
        throw new Error(result.message || `Request failed (${response.status})`);
    return result;
}
function formatDate(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}
function showAlert(message, success = false) {
    const alert = document.getElementById("notification-alert");
    alert.className = `alert ${success ? "alert-success" : "alert-danger"}`;
    alert.textContent = message;
    alert.hidden = false;
}
function updateUnreadBadge(count) {
    const badge = document.getElementById("notifications-unread-count");
    if (!badge)
        return;
    badge.textContent = String(count);
    badge.hidden = count === 0;
}
async function refreshUnreadCount() {
    try {
        const result = await request("/notifications");
        const unreadCount = (result.notifications || []).filter((notification) => !notification.is_read).length;
        updateUnreadBadge(unreadCount);
    }
    catch {
        return;
    }
}
function actionButton(label, className) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `btn btn-small ${className}`;
    button.textContent = label;
    return button;
}
function renderNotifications(notifications) {
    notificationList.replaceChildren();
    for (const notification of notifications) {
        const item = document.createElement("article");
        item.className = `notification-item${notification.is_read ? "" : " notification-unread"}`;
        item.addEventListener("click", () => void openNotification(notification.id));
        const content = document.createElement("div");
        const title = document.createElement("strong");
        title.textContent = notification.title;
        const message = document.createElement("p");
        message.textContent = notification.message;
        const date = document.createElement("small");
        date.textContent = formatDate(notification.created_at);
        content.append(title, message, date);
        const actions = document.createElement("div");
        actions.className = "notification-actions";
        const status = document.createElement("span");
        status.className = `badge ${notification.is_read ? "badge-success" : "badge-warning"}`;
        status.textContent = notification.is_read ? "Read" : "Unread";
        actions.appendChild(status);
        if (!notification.is_read) {
            const readButton = actionButton("Mark read", "btn-secondary");
            readButton.addEventListener("click", (event) => {
                event.stopPropagation();
                void markNotificationRead(notification.id);
            });
            actions.appendChild(readButton);
        }
        const deleteButton = actionButton("Delete", "btn-danger");
        deleteButton.addEventListener("click", (event) => {
            event.stopPropagation();
            void deleteNotification(notification.id);
        });
        actions.appendChild(deleteButton);
        item.append(content, actions);
        notificationList.appendChild(item);
    }
}
async function loadNotifications() {
    loadingState.hidden = false;
    emptyState.hidden = true;
    errorState.hidden = true;
    notificationList.hidden = true;
    try {
        const result = await request("/notifications");
        const notifications = result.notifications || [];
        loadingState.hidden = true;
        if (!notifications.length) {
            emptyState.hidden = false;
            return;
        }
        renderNotifications(notifications);
        notificationList.hidden = false;
    }
    catch (error) {
        loadingState.hidden = true;
        errorState.textContent =
            error instanceof Error ? error.message : "Unable to load notifications.";
        errorState.hidden = false;
    }
}
async function openNotification(id) {
    selectedNotificationID = id;
    try {
        const result = await request(`/notifications/${id}`);
        const notification = result.notification;
        if (!notification)
            throw new Error("Notification not found.");
        document.getElementById("notification-view-title").textContent = notification.title;
        document.getElementById("notification-view-date").textContent = formatDate(notification.created_at);
        document.getElementById("notification-view-type").textContent = notification.type;
        document.getElementById("notification-view-status").textContent = notification.is_read ? "Read" : "Unread";
        document.getElementById("notification-view-message").textContent = notification.message;
        viewModal.classList.remove("hidden");
        document.body.style.overflow = "hidden";
        if (!notification.is_read) {
            await markNotificationRead(id);
            document.getElementById("notification-view-status").textContent = "Read";
        }
    }
    catch (error) {
        showAlert(error instanceof Error ? error.message : "Unable to open notification.");
    }
}
async function markNotificationRead(id) {
    try {
        await request(`/notifications/${id}/read`, {
            method: "PATCH",
        });
        await loadNotifications();
        await refreshUnreadCount();
    }
    catch (error) {
        showAlert(error instanceof Error
            ? error.message
            : "Unable to mark notification as read.");
    }
}
async function deleteNotification(id) {
    if (!window.confirm("Delete this notification?"))
        return;
    try {
        await request(`/notifications/${id}`, {
            method: "DELETE",
        });
        if (selectedNotificationID === id)
            closeViewModal();
        showAlert("Notification deleted successfully.", true);
        await loadNotifications();
        await refreshUnreadCount();
    }
    catch (error) {
        showAlert(error instanceof Error ? error.message : "Unable to delete notification.", false);
    }
}
async function loadRecipients() {
    recipientSelect.replaceChildren(new Option("Loading recipients...", ""));
    recipientSelect.disabled = true;
    try {
        const result = await request("/messages/recipients");
        recipientSelect.replaceChildren(new Option("Select recipient", ""));
        for (const recipient of result.data || []) {
            const label = recipient.email
                ? `${recipient.name} (${recipient.email})`
                : recipient.name;
            recipientSelect.appendChild(new Option(label, recipient.id));
        }
        if (!result.data?.length)
            recipientSelect.replaceChildren(new Option("No recipients available", ""));
    }
    catch (error) {
        recipientSelect.replaceChildren(new Option("Failed to load recipients", ""));
        showAlert(error instanceof Error ? error.message : "Unable to load recipients.");
    }
    finally {
        recipientSelect.disabled = false;
    }
}
function openCreateModal() {
    createModal.classList.remove("hidden");
    document.body.style.overflow = "hidden";
    createFeedback.hidden = true;
    void loadRecipients();
}
function closeCreateModal() {
    createModal.classList.add("hidden");
    document.body.style.overflow = "";
    createForm.reset();
    createFeedback.hidden = true;
    createSubmit.disabled = false;
    createSubmit.textContent = "Create Notification";
}
function closeViewModal() {
    viewModal.classList.add("hidden");
    document.body.style.overflow = "";
    selectedNotificationID = "";
}
async function submitCreate(event) {
    event.preventDefault();
    const formData = new FormData(createForm);
    const payload = {
        user_id: String(formData.get("user_id") || "").trim(),
        title: String(formData.get("title") || "").trim(),
        message: String(formData.get("message") || "").trim(),
        type: String(formData.get("type") || "").trim(),
    };
    if (!payload.user_id || !payload.title || !payload.message || !payload.type) {
        createFeedback.textContent =
            "Recipient, title, message, and type are required.";
        createFeedback.className = "form-message error";
        createFeedback.hidden = false;
        return;
    }
    createSubmit.disabled = true;
    createSubmit.textContent = "Creating...";
    try {
        await request("/notifications", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload),
        });
        closeCreateModal();
        showAlert("Notification created successfully.", true);
        await refreshUnreadCount();
    }
    catch (error) {
        createFeedback.textContent =
            error instanceof Error ? error.message : "Unable to create notification.";
        createFeedback.className = "form-message error";
        createFeedback.hidden = false;
        createSubmit.disabled = false;
        createSubmit.textContent = "Create Notification";
    }
}
document
    .getElementById("create-notification-btn")
    ?.addEventListener("click", openCreateModal);
document
    .getElementById("create-notification-close")
    ?.addEventListener("click", closeCreateModal);
document
    .getElementById("create-notification-cancel")
    ?.addEventListener("click", closeCreateModal);
document
    .getElementById("notification-view-close")
    ?.addEventListener("click", closeViewModal);
document
    .getElementById("notification-view-cancel")
    ?.addEventListener("click", closeViewModal);
document
    .getElementById("notification-view-delete")
    ?.addEventListener("click", () => {
    if (selectedNotificationID)
        void deleteNotification(selectedNotificationID);
});
createModal
    .querySelector(".modal-backdrop")
    ?.addEventListener("click", closeCreateModal);
viewModal
    .querySelector(".modal-backdrop")
    ?.addEventListener("click", closeViewModal);
createForm.addEventListener("submit", (event) => void submitCreate(event));
document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape")
        return;
    if (!createModal.classList.contains("hidden"))
        closeCreateModal();
    else if (!viewModal.classList.contains("hidden"))
        closeViewModal();
});
void loadNotifications();
void refreshUnreadCount();
export {};
//# sourceMappingURL=notifications.js.map