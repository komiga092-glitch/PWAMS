const menuButton = document.querySelector<HTMLElement>("[data-menu-button]");

const sidebar = document.querySelector<HTMLElement>(".sidebar");

const overlay = document.querySelector<HTMLElement>("[data-overlay]");

function openSidebar(): void {
  sidebar?.classList.add("open");
  overlay?.classList.add("show");
}

function closeSidebar(): void {
  sidebar?.classList.remove("open");
  overlay?.classList.remove("show");
}

function toggleSidebar(): void {
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

type AuthenticatedUser = {
  username?: string;
  email?: string;
  role?: string;
};

function updateProfile(user: AuthenticatedUser): void {
  const username = user.username ?? "Account";
  const role = user.role ?? "";
  const avatar = document.querySelector<HTMLElement>("[data-profile-avatar]");
  const name = document.querySelector<HTMLElement>("[data-profile-name]");
  const usernameElement = document.querySelector<HTMLElement>(
    "[data-profile-username]",
  );
  const roleElement = document.querySelector<HTMLElement>(
    "[data-profile-role]",
  );

  if (avatar) avatar.textContent = username.charAt(0).toUpperCase() || "U";
  if (name) name.textContent = username;
  if (usernameElement) usernameElement.textContent = `Username: ${username}`;
  if (roleElement) roleElement.textContent = role ? `Role: ${role}` : "";
}

function updateRoleVisibility(): void {
  document.querySelectorAll<HTMLElement>("[data-roles]").forEach((element) => {
    const roles = element.dataset.roles?.split(",") ?? [];
    element.hidden = !currentRole || !roles.includes(currentRole);
  });
}

async function applyRoleVisibility(): Promise<void> {
  try {
    const response = await fetch("/auth/me", {
      credentials: "same-origin",
    });
    if (!response.ok) {
      currentRole = "";
      updateRoleVisibility();
      return;
    }

    const result = (await response.json()) as AuthenticatedUser;
    currentRole = result.role ?? "";
    updateProfile(result);
    updateRoleVisibility();
  } catch {
    currentRole = "";
    updateRoleVisibility();
  }
}

// =========================
// HTMX Configuration
// =========================

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp(`(^| )${name}=([^;]+)`));
  return match ? decodeURIComponent(match[2]) : null;
}

interface ConfigRequestEvent {
  detail: {
    headers: Record<string, string>;
  };
}

function hasHtmx(): boolean {
  return Boolean((window as unknown as Record<string, unknown>)["htmx"]);
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
function setupCsrfFetch(): void {
  const globalWindow = window as Window &
    typeof globalThis & { __pwamsCsrfFetchPatched?: boolean };
  if (globalWindow.__pwamsCsrfFetchPatched) return;
  globalWindow.__pwamsCsrfFetchPatched = true;

  const originalFetch = window.fetch.bind(window);

  const patchedFetch = (
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> => {
    try {
      const method = String(
        init?.method ??
          (typeof input !== "string" && !(input instanceof URL)
            ? input.method
            : "GET"),
      ).toUpperCase();

      if (!["GET", "HEAD", "OPTIONS"].includes(method)) {
        const url =
          typeof input === "string" || input instanceof URL
            ? String(input)
            : input.url;

        if (url.startsWith("/") || url.startsWith(window.location.origin)) {
          const headers = new Headers(
            init?.headers ??
              (typeof input !== "string" && !(input instanceof URL)
                ? input.headers
                : undefined),
          );
          const token = getCookie("pwams_csrf");
          if (token && !headers.has("X-CSRF-Token")) {
            headers.set("X-CSRF-Token", token);
          }

          if (typeof input !== "string" && !(input instanceof URL) && !init) {
            return originalFetch(new Request(input, { headers }));
          }
          return originalFetch(input, { ...init, headers });
        }
      } else if (typeof input !== "string" && !(input instanceof URL)) {
        return originalFetch(new Request(input), init);
      }
    } catch {
      // Fall through to the original request rather than breaking the
      // caller if header preparation fails unexpectedly.
    }
    return originalFetch(
      typeof input === "string" || input instanceof URL ? input : new Request(input),
      init,
    );
  };

  window.fetch = patchedFetch;
}

/**
 * Sidebar navigation should open every page at the top consistently.
 * The default "auto" restoration re-applies stale scroll offsets after
 * location.reload() (which most CRUD flows trigger after success),
 * leaving users mid-page at inconsistent positions.
 */
function setupScrollRestoration(): void {
  if ("scrollRestoration" in history) {
    history.scrollRestoration = "manual";
  }
}

function setupHtmx(): void {
  if (!hasHtmx()) return;

  document.addEventListener("htmx:configRequest", (event: Event) => {
    const configEvent = event as CustomEvent<ConfigRequestEvent["detail"]>;
    const token =
      getCookie("pwams_csrf") ||
      document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]')
        ?.content ||
      "";
    if (token) {
      configEvent.detail.headers["X-CSRF-Token"] = token;
    }
  });

  document.addEventListener("htmx:responseError", (event: Event) => {
    const e = event as CustomEvent;
    const target = e.detail?.target as HTMLElement | null;
    if (target) {
      target.innerHTML =
        '<div class="alert alert-danger">An error occurred. Please try again.</div>';
    }
  });

  document.addEventListener("htmx:sendError", (event: Event) => {
    const e = event as CustomEvent;
    const target = e.detail?.target as HTMLElement | null;
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
function setupFormCsrf(): void {
  document.addEventListener(
    "submit",
    (event) => {
      const form = event.target as HTMLFormElement;
      if (!form || form.method.toLowerCase() !== "post") {
        return;
      }
      if (form.querySelector<HTMLInputElement>('input[name="_csrf"]')) {
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
    },
    true,
  );
}

// =========================
// Auto-dismiss alerts
// =========================

function setupAutoDismiss(): void {
  document.querySelectorAll<HTMLElement>("[data-auto-dismiss]").forEach((el) => {
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
