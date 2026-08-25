"use strict";
const messageViewLabels = {
    inbox: "Inbox",
    sent: "Sent",
    unread: "Unread",
};
let currentView = "inbox";
let currentPage = 1;
let totalPages = 1;
let selectedMessageID = "";
let recipientsLoaded = false;
const element = (id) => document.getElementById(id);
const listTitle = element("message-list-title");
const listCount = element("message-list-count");
const table = element("messages-table");
const tableBody = element("messages-table-body");
const loading = element("messages-loading");
const empty = element("messages-empty");
const errorState = element("messages-error");
const pagination = element("messages-pagination");
const pageInfo = element("messages-page-info");
const previousButton = element("messages-previous");
const nextButton = element("messages-next");
const composeModal = element("compose-message-modal");
const composeForm = element("compose-message-form");
const composeFeedback = element("compose-message-feedback");
const composeSubmit = element("compose-message-submit");
const recipientSelect = element("message-recipient");
const viewModal = element("message-view-modal");
function setAlert(message, kind) {
    const alert = element("message-alert");
    alert.textContent = message;
    alert.className = `alert alert-${kind === "success" ? "success" : "danger"}`;
    alert.hidden = false;
}
function clearAlert() {
    const alert = element("message-alert");
    alert.hidden = true;
    alert.textContent = "";
}
function apiError(response, payload) {
    return new Error(payload.message || `Request failed (${response.status})`);
}
async function fetchJson(url, options) {
    const response = await fetch(url, {
        credentials: "same-origin",
        headers: { Accept: "application/json", ...options?.headers },
        ...options,
    });
    const payload = (await response.json().catch(() => ({})));
    if (!response.ok)
        throw apiError(response, payload);
    return payload;
}
function displayUser(user) {
    return user?.username || user?.name || user?.email || "Unknown user";
}
function formatMessageDate(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}
function statusBadge(isRead) {
    const badge = document.createElement("span");
    badge.className = `badge ${isRead ? "badge-success" : "badge-warning"}`;
    badge.textContent = isRead ? "Read" : "Unread";
    return badge;
}
function actionButton(label, className) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `btn btn-small ${className}`;
    button.textContent = label;
    return button;
}
function renderMessages(messages) {
    tableBody.replaceChildren();
    for (const message of messages) {
        const row = document.createElement("tr");
        const person = document.createElement("td");
        person.textContent =
            currentView === "sent"
                ? displayUser(message.recipient)
                : displayUser(message.sender);
        const subject = document.createElement("td");
        subject.textContent = message.subject;
        const date = document.createElement("td");
        date.textContent = formatMessageDate(message.created_at);
        const status = document.createElement("td");
        status.appendChild(statusBadge(message.is_read));
        const actions = document.createElement("td");
        actions.className = "table-actions";
        const viewButton = actionButton("View", "btn-secondary");
        viewButton.addEventListener("click", () => void openMessage(message.id));
        const deleteButton = actionButton("Delete", "btn-danger");
        deleteButton.addEventListener("click", () => void deleteMessage(message.id));
        actions.append(viewButton, deleteButton);
        row.append(person, subject, date, status, actions);
        tableBody.appendChild(row);
    }
}
async function loadMessages(page = 1) {
    currentPage = page;
    listTitle.textContent = messageViewLabels[currentView];
    element("message-person-heading").textContent =
        currentView === "sent" ? "Recipient" : "Sender";
    loading.hidden = false;
    empty.hidden = true;
    errorState.hidden = true;
    table.hidden = true;
    pagination.hidden = true;
    try {
        const endpoint = currentView === "inbox" ? "inbox" : currentView;
        const result = await fetchJson(`/messages/${endpoint}?page=${page}&page_size=20`);
        const messages = result.data || [];
        const pageData = result.pagination;
        totalPages = pageData?.total_pages || 1;
        listCount.textContent = `${pageData?.total_items || messages.length} Records`;
        loading.hidden = true;
        if (messages.length === 0) {
            empty.textContent = `No ${messageViewLabels[currentView].toLowerCase()} messages found.`;
            empty.hidden = false;
            return;
        }
        renderMessages(messages);
        table.hidden = false;
        if (totalPages > 1) {
            pageInfo.textContent = `Page ${pageData?.page || page} of ${totalPages}`;
            previousButton.disabled = page <= 1;
            nextButton.disabled = page >= totalPages;
            pagination.hidden = false;
        }
    }
    catch (requestError) {
        loading.hidden = true;
        errorState.textContent =
            requestError instanceof Error
                ? requestError.message
                : "Unable to load messages.";
        errorState.hidden = false;
    }
}
async function updateUnreadCount() {
    try {
        const result = await fetchJson("/messages/unread/count");
        const count = result.unread_count || 0;
        for (const id of ["messages-unread-count", "messages-page-unread-count"]) {
            const badge = document.getElementById(id);
            if (!badge)
                continue;
            badge.textContent = String(count);
            badge.hidden = count === 0;
        }
    }
    catch {
        return;
    }
}
async function loadRecipients() {
    if (recipientsLoaded)
        return;
    recipientSelect.replaceChildren(new Option("Loading recipients...", ""));
    recipientSelect.disabled = true;
    try {
        const result = await fetchJson("/messages/recipients");
        recipientSelect.replaceChildren(new Option("Select recipient", ""));
        for (const recipient of result.data || []) {
            const label = recipient.email
                ? `${recipient.name} (${recipient.email})`
                : recipient.name;
            recipientSelect.appendChild(new Option(label, recipient.id));
        }
        if (!result.data?.length) {
            recipientSelect.replaceChildren(new Option("No recipients available", ""));
        }
        recipientsLoaded = true;
    }
    catch (requestError) {
        recipientSelect.replaceChildren(new Option("Failed to load recipients", ""));
        setAlert(requestError instanceof Error
            ? requestError.message
            : "Failed to load recipients", "error");
    }
    finally {
        recipientSelect.disabled = false;
    }
}
function closeCompose() {
    composeModal.classList.add("hidden");
    document.body.style.overflow = "";
    composeForm.reset();
    composeFeedback.hidden = true;
    composeSubmit.disabled = false;
    composeSubmit.textContent = "Send Message";
}
function openCompose() {
    clearAlert();
    composeModal.classList.remove("hidden");
    document.body.style.overflow = "hidden";
    void loadRecipients();
    element("message-subject").focus();
}
async function submitCompose(event) {
    event.preventDefault();
    const formData = new FormData(composeForm);
    const recipientID = String(formData.get("recipient_id") || "").trim();
    const subject = String(formData.get("subject") || "").trim();
    const body = String(formData.get("body") || "").trim();
    if (!recipientID || !subject || !body) {
        composeFeedback.textContent =
            "Recipient, subject, and message body are required.";
        composeFeedback.className = "form-message error";
        composeFeedback.hidden = false;
        return;
    }
    composeSubmit.disabled = true;
    composeSubmit.textContent = "Sending...";
    composeFeedback.hidden = true;
    try {
        await fetchJson("/messages", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ recipient_id: recipientID, subject, body }),
        });
        closeCompose();
        setAlert("Message sent successfully.", "success");
        await loadMessages(currentView === "sent" ? currentPage : 1);
        await updateUnreadCount();
    }
    catch (requestError) {
        composeFeedback.textContent =
            requestError instanceof Error
                ? requestError.message
                : "Unable to send message.";
        composeFeedback.className = "form-message error";
        composeFeedback.hidden = false;
        composeSubmit.disabled = false;
        composeSubmit.textContent = "Send Message";
    }
}
async function openMessage(id) {
    selectedMessageID = id;
    try {
        const result = await fetchJson(`/messages/${id}`);
        const message = result.data;
        if (!message)
            throw new Error("Message was not found.");
        element("message-view-subject").textContent = message.subject;
        element("message-view-date").textContent = formatMessageDate(message.created_at);
        element("message-view-sender").textContent = displayUser(message.sender);
        element("message-view-recipient").textContent = displayUser(message.recipient);
        element("message-view-status").textContent = message.is_read
            ? "Read"
            : "Unread";
        element("message-view-body").textContent = message.body;
        viewModal.classList.remove("hidden");
        document.body.style.overflow = "hidden";
        if (!message.is_read) {
            await fetchJson(`/messages/${id}/read`, {
                method: "PATCH",
            });
            element("message-view-status").textContent = "Read";
            await updateUnreadCount();
            await loadMessages(currentPage);
        }
    }
    catch (requestError) {
        setAlert(requestError instanceof Error
            ? requestError.message
            : "Unable to open message.", "error");
    }
}
function closeMessageView() {
    viewModal.classList.add("hidden");
    document.body.style.overflow = "";
    selectedMessageID = "";
}
async function deleteMessage(id) {
    if (!window.confirm("Delete this message?"))
        return;
    try {
        await fetchJson(`/messages/${id}`, {
            method: "DELETE",
        });
        if (selectedMessageID === id)
            closeMessageView();
        setAlert("Message deleted successfully.", "success");
        await loadMessages(currentPage);
        await updateUnreadCount();
    }
    catch (requestError) {
        setAlert(requestError instanceof Error
            ? requestError.message
            : "Unable to delete message.", "error");
    }
}
document
    .querySelectorAll("[data-message-view]")
    .forEach((button) => {
    button.addEventListener("click", () => {
        currentView = button.dataset.messageView;
        document
            .querySelectorAll("[data-message-view]")
            .forEach((tab) => {
            const active = tab === button;
            tab.classList.toggle("active", active);
            tab.setAttribute("aria-selected", String(active));
        });
        void loadMessages(1);
    });
});
element("compose-message-btn").addEventListener("click", openCompose);
element("compose-message-close").addEventListener("click", closeCompose);
element("compose-message-cancel").addEventListener("click", closeCompose);
element("message-view-close").addEventListener("click", closeMessageView);
element("message-view-cancel").addEventListener("click", closeMessageView);
element("message-view-delete").addEventListener("click", () => {
    if (selectedMessageID)
        void deleteMessage(selectedMessageID);
});
composeForm.addEventListener("submit", (event) => void submitCompose(event));
composeModal.addEventListener("click", (event) => {
    if (event.target === composeModal)
        closeCompose();
});
viewModal.addEventListener("click", (event) => {
    if (event.target === viewModal)
        closeMessageView();
});
previousButton.addEventListener("click", () => void loadMessages(currentPage - 1));
nextButton.addEventListener("click", () => void loadMessages(currentPage + 1));
document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape")
        return;
    if (!composeModal.classList.contains("hidden"))
        closeCompose();
    else if (!viewModal.classList.contains("hidden"))
        closeMessageView();
});
void loadMessages();
void updateUnreadCount();
//# sourceMappingURL=messages.js.map