const menuButton =
    document.querySelector<HTMLElement>("[data-menu-button]");

const sidebar =
    document.querySelector<HTMLElement>(".sidebar");

const overlay =
    document.querySelector<HTMLElement>("[data-overlay]");


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


console.log("PWAMS TypeScript loaded successfully");