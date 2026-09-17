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
function updateProfile(user) {
    const username = user.username ?? "Account";
    const role = user.role ?? "";
    const avatar = document.querySelector("[data-profile-avatar]");
    const name = document.querySelector("[data-profile-name]");
    const usernameElement = document.querySelector("[data-profile-username]");
    const roleElement = document.querySelector("[data-profile-role]");
    if (avatar)
        avatar.textContent = username.charAt(0).toUpperCase() || "U";
    if (name)
        name.textContent = username;
    if (usernameElement)
        usernameElement.textContent = `Username: ${username}`;
    if (roleElement)
        roleElement.textContent = role ? `Role: ${role}` : "";
}
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
        if (!response.ok) {
            currentRole = "";
            updateRoleVisibility();
            return;
        }
        const result = (await response.json());
        currentRole = result.role ?? "";
        updateProfile(result);
        updateRoleVisibility();
    }
    catch {
        currentRole = "";
        updateRoleVisibility();
    }
}
// =========================
// HTMX Configuration
// =========================
function getCookie(name) {
    const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
    return match ? decodeURIComponent(match[2]) : null;
}
function hasHtmx() {
    return Boolean(window["htmx"]);
}
/**
 * Adds the double-submit CSRF token to every same-origin unsafe
 * (POST/PUT/PATCH/DELETE) fetch request issued by any script on the
 * page (page controllers and inline template scripts alike). The
 * server-side EnsureCSRF middleware rejects unsafe requests without
 * this header, so all online CRUD mutations depend on it.
 *
 * Tokens set explicitly by callers (e.g. the offline sync module) are
 * left untouched.
 */
function setupCsrfFetch() {
    const globalWindow = window;
    if (globalWindow.__pwamsCsrfFetchPatched)
        return;
    globalWindow.__pwamsCsrfFetchPatched = true;
    const originalFetch = window.fetch.bind(window);
    const patchedFetch = (input, init) => {
        try {
            const method = String(init?.method ??
                (typeof input !== "string" && !(input instanceof URL)
                    ? input.method
                    : "GET")).toUpperCase();
            if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
                const url = typeof input === "string" || input instanceof URL
                    ? String(input)
                    : input.url;
                if (url.startsWith("/") || url.startsWith(window.location.origin)) {
                    const headers = new Headers(init?.headers ??
                        (typeof input !== "string" && !(input instanceof URL)
                            ? input.headers
                            : undefined));
                    const token = getCookie("pwams_csrf");
                    if (token && !headers.has("X-CSRF-Token")) {
                        headers.set("X-CSRF-Token", token);
                    }
                    if (typeof input !== "string" && !(input instanceof URL) && !init) {
                        return originalFetch(new Request(input, { headers }));
                    }
                    return originalFetch(input, { ...init, headers });
                }
            }
            else if (typeof input !== "string" && !(input instanceof URL)) {
                return originalFetch(new Request(input), init);
            }
        }
        catch {
            // Fall through to the original request rather than breaking the
            // caller if header preparation fails unexpectedly.
        }
        return originalFetch(typeof input === "string" || input instanceof URL ? input : new Request(input), init);
    };
    window.fetch = patchedFetch;
}
/**
 * Sidebar navigation should open every page at the top consistently.
 * The default "auto" restoration re-applies stale scroll offsets after
 * location.reload() (which most CRUD flows trigger after success),
 * leaving users mid-page at inconsistent positions.
 */
function setupScrollRestoration() {
    if ("scrollRestoration" in history) {
        history.scrollRestoration = "manual";
    }
}
function setupHtmx() {
    if (!hasHtmx())
        return;
    // CSP hardening: htmx must never compile expressions with eval/new
    // Function (the runtime reason 'unsafe-eval' existed). All dynamic
    // behaviour is wired through server-rendered attributes and the
    // event listeners below instead of hx-on:* script attributes.
    const htmxGlobal = window["htmx"];
    if (htmxGlobal?.config) {
        htmxGlobal.config.allowEval = false;
    }
    document.addEventListener("htmx:configRequest", (event) => {
        const configEvent = event;
        const token = getCookie("pwams_csrf") ||
            document.querySelector('meta[name="csrf-token"]')
                ?.content ||
            "";
        if (token) {
            configEvent.detail.headers["X-CSRF-Token"] = token;
        }
    });
    // Person modal wiring (replaces the removed inline hx-on::after-request
    // script attributes so script-src can drop 'unsafe-eval').
    document.addEventListener("htmx:afterRequest", (event) => {
        const detail = event.detail;
        const elt = detail?.elt;
        if (!elt)
            return;
        if (elt.id === "add-person-btn" && detail.successful) {
            document.getElementById("person-modal")?.classList.remove("hidden");
        }
        if (elt.id === "person-create-form" && detail.successful) {
            elt.closest(".modal")?.classList.add("hidden");
        }
    });
    document.addEventListener("htmx:responseError", (event) => {
        const e = event;
        const target = e.detail?.target;
        if (target) {
            target.innerHTML =
                '<div class="alert alert-danger">An error occurred. Please try again.</div>';
        }
    });
    document.addEventListener("htmx:sendError", (event) => {
        const e = event;
        const target = e.detail?.target;
        if (target) {
            target.innerHTML =
                '<div class="alert alert-danger">Network error. Please check your connection.</div>';
        }
    });
}
/**
 * Injects a hidden _csrf field into any HTML form that does not
 * already carry one, right before submission. Covers plain form POSTs
 * (login, logout, profile, CRUD forms) in addition to the header-based
 * protection applied to HTMX and fetch requests.
 */
function setupFormCsrf() {
    document.addEventListener("submit", (event) => {
        const form = event.target;
        if (!form || form.method.toLowerCase() !== "post") {
            return;
        }
        if (form.querySelector('input[name="_csrf"]')) {
            return;
        }
        const token = getCookie("pwams_csrf");
        if (!token) {
            return;
        }
        const input = document.createElement("input");
        input.type = "hidden";
        input.name = "_csrf";
        input.value = token;
        form.appendChild(input);
    }, true);
}
// =========================
// Auto-dismiss alerts
// =========================
function setupAutoDismiss() {
    document.querySelectorAll("[data-auto-dismiss]").forEach((el) => {
        setTimeout(() => {
            el.style.display = "none";
        }, 5000);
    });
}
// =========================
// Init
// =========================
updateRoleVisibility();
void applyRoleVisibility();
new MutationObserver(updateRoleVisibility).observe(document.body, {
    childList: true,
    subtree: true,
});
setupCsrfFetch();
setupScrollRestoration();
setupHtmx();
setupFormCsrf();
setupAutoDismiss();
//# sourceMappingURL=app.js.map