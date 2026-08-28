interface RevenueResponse {
  success?: boolean;
  message?: string;
  error?: string;
}

const revenueModal = document.getElementById("revenueModal");
const revenueForm = document.getElementById(
  "revenue-form",
) as HTMLFormElement | null;

function openRevenueModal(): void {
  revenueModal?.classList.remove("hidden");
  document.body.style.overflow = "hidden";
}

function closeRevenueModal(): void {
  revenueModal?.classList.add("hidden");
  document.body.style.overflow = "";
  revenueForm?.reset();
}

revenueModal?.addEventListener("click", (event: MouseEvent) => {
  if (event.target === event.currentTarget) closeRevenueModal();
});

document.addEventListener("keydown", (event: KeyboardEvent) => {
  if (
    event.key === "Escape" &&
    revenueModal &&
    !revenueModal.classList.contains("hidden")
  ) {
    closeRevenueModal();
  }
});

async function submitRevenue(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  if (!revenueForm) return;

  const submitButton = revenueForm.querySelector<HTMLButtonElement>(
    'button[type="submit"]',
  );
  if (submitButton) submitButton.disabled = true;
  const data = Object.fromEntries(new FormData(revenueForm)) as Record<
    string,
    string
  >;
  const requestBody = { ...data, amount: Number(data.amount || 0) };

  try {
    if (!navigator.onLine) {
      const { savePendingMutation, offlineSuccessMessage } = await import(
        String("/static/js/offline/mutations.js")
      );
      await savePendingMutation("revenue", "CREATE", requestBody);
      alert(offlineSuccessMessage("revenue", "CREATE"));
      closeRevenueModal();
      return;
    }

    const response = await fetch("/revenue", {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify(requestBody),
    });
    const result = (await response.json()) as RevenueResponse;
    if (!response.ok || result.success === false) {
      throw new Error(
        result.message || result.error || "Unable to save revenue record.",
      );
    }
    alert(result.message || "Revenue record created successfully.");
    window.location.reload();
  } catch (error: unknown) {
    alert(
      error instanceof Error ? error.message : "Unable to save revenue record.",
    );
  } finally {
    if (submitButton) submitButton.disabled = false;
  }
}

async function deleteRevenueRecord(id: string): Promise<void> {
  if (!confirm("Delete this revenue record?")) return;
  try {
    const response = await fetch(`/revenue/${id}`, {
      method: "DELETE",
      credentials: "include",
      headers: { Accept: "application/json" },
    });
    const result = (await response.json()) as RevenueResponse;
    if (!response.ok || result.success === false) {
      throw new Error(
        result.message || result.error || "Unable to delete revenue record.",
      );
    }
    alert(result.message || "Revenue record deleted successfully.");
    window.location.reload();
  } catch (error: unknown) {
    alert(
      error instanceof Error
        ? error.message
        : "Unable to delete revenue record.",
    );
  }
}

async function loadRevenueSummaries(): Promise<void> {
  const target = document.getElementById("revenue-summaries");
  if (!target || !navigator.onLine) return;
  try {
    const response = await fetch("/revenue/summaries", {
      credentials: "include",
    });
    const result = (await response.json()) as {
      data?: Array<{
        period: string;
        income: number;
        expenses: number;
        net: number;
      }>;
      message?: string;
    };
    if (!response.ok)
      throw new Error(result.message || "Unable to load summaries");
    target.innerHTML = (result.data || [])
      .map(
        (summary) =>
          `<div><strong>${summary.period}</strong>: ${summary.income.toFixed(2)} income, ${summary.expenses.toFixed(2)} expenses, ${summary.net.toFixed(2)} net</div>`,
      )
      .join("");
  } catch (error: unknown) {
    target.textContent =
      error instanceof Error ? error.message : "Unable to load summaries";
  }
}

window.addEventListener("DOMContentLoaded", () => {
  void loadRevenueSummaries();
});

(window as Window & typeof globalThis).openRevenueModal = openRevenueModal;
(window as Window & typeof globalThis).closeRevenueModal = closeRevenueModal;
(window as Window & typeof globalThis).submitRevenue = submitRevenue;
(window as Window & typeof globalThis).deleteRevenueRecord =
  deleteRevenueRecord;
