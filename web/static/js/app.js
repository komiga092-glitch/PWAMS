"use strict";
const menuButton = document.querySelector("[data-menu-button]");
const sidebar = document.querySelector(".sidebar");
const overlay = document.querySelector("[data-overlay]");
function openSidebar() {
    sidebar?.classList.add("open");
    overlay?.classList.add("show");
}
function closeSidebar() {
    sidebar?.classList.remove("open");
    overlay?.classList.remove("show");
}
function toggleSidebar() {
    if (!sidebar) {
        return;
    }
    if (sidebar.classList.contains("open")) {
        closeSidebar();
        return;
    }
    openSidebar();
}
menuButton?.addEventListener("click", () => {
    toggleSidebar();
});
overlay?.addEventListener("click", () => {
    closeSidebar();
});
let currentRole = "";
function updateRoleVisibility() {
    document.querySelectorAll("[data-roles]").forEach((element) => {
        const roles = element.dataset.roles?.split(",") ?? [];
        element.hidden = !currentRole || !roles.includes(currentRole);
    });
}
async function applyRoleVisibility() {
    try {
        const response = await fetch("/auth/me", {
            credentials: "same-origin",
        });
        if (!response.ok)
            return;
        const result = (await response.json());
        currentRole = result.role ?? "";
        updateRoleVisibility();
    }
    catch {
        return;
    }
}
void applyRoleVisibility();
new MutationObserver(updateRoleVisibility).observe(document.body, {
    childList: true,
    subtree: true,
});
console.log("PWAMS TypeScript loaded successfully");
//# sourceMappingURL=app.js.map