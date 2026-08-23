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

function updateRoleVisibility(): void {
  document.querySelectorAll<HTMLElement>("[data-roles]").forEach((element) => {
    const roles = element.dataset.roles?.split(",") ?? [];
    element.hidden = !currentRole || !roles.includes(currentRole);
  });
}

async function applyRoleVisibility(): Promise<void> {
  const response = await fetch("/auth/me", { credentials: "same-origin" });
  if (!response.ok) return;

  const result = (await response.json()) as { role?: string };
  currentRole = result.role ?? "";
  updateRoleVisibility();
}

void applyRoleVisibility();
new MutationObserver(updateRoleVisibility).observe(document.body, {
  childList: true,
  subtree: true,
});

console.log("PWAMS TypeScript loaded successfully");
