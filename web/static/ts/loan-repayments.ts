function closeRepaymentModal(): void {
  document.getElementById("repaymentModal")?.classList.add("hidden");
  document.body.style.overflow = "";
  (
    document.getElementById("repayment-form") as HTMLFormElement | null
  )?.reset();
}

function openRepaymentModal(): void {
  document.getElementById("repaymentModal")?.classList.remove("hidden");
  document.body.style.overflow = "hidden";
}

document
  .getElementById("repaymentModal")
  ?.addEventListener("click", (event: MouseEvent) => {
    if (event.target === event.currentTarget) closeRepaymentModal();
  });

document.addEventListener("keydown", (event: KeyboardEvent) => {
  const modal = document.getElementById("repaymentModal");
  if (event.key === "Escape" && modal && !modal.classList.contains("hidden"))
    closeRepaymentModal();
});

async function loadRepaymentLoans(): Promise<void> {
  const select = document.getElementById(
    "repaymentLoanID",
  ) as HTMLSelectElement | null;
  if (!select || select.options.length > 1) return;

  if (!navigator.onLine) {
    const { readOfflineStore } = await import("./offline-data.js");
    const loans = await readOfflineStore<{
      id: string;
      loan_amount: number;
      is_deleted?: boolean;
    }>("loans");
    for (const loan of loans.filter((item) => !item.is_deleted)) {
      const option = document.createElement("option");
      option.value = loan.id;
      option.textContent = `${loan.id} - ${loan.loan_amount}`;
      select.appendChild(option);
    }
    return;
  }

  const response = await fetch("/loans?page_size=100", {
    credentials: "same-origin",
  });
  if (!response.ok) return;
  const result = await response.json();
  for (const loan of result.data?.loans || []) {
    const option = document.createElement("option");
    option.value = loan.id;
    option.textContent = `${loan.id} - ${loan.loan_amount}`;
    select.appendChild(option);
  }
}

document.addEventListener("DOMContentLoaded", () => {
  void loadRepaymentLoans();
});

async function submitRepayment(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  const form = event.currentTarget as HTMLFormElement;
  const data = Object.fromEntries(new FormData(form)) as Record<string, string>;
  const requestBody = {
    ...data,
    installment_number: Number(data.installment_number || 0),
    amount: Number(data.amount || 0),
  };

  try {
    if (!navigator.onLine) {
      const { savePendingMutation, offlineSuccessMessage } = await import(
        String("/static/js/offline/mutations.js")
      );
      await savePendingMutation("loan_repayment", "CREATE", requestBody);
      alert(offlineSuccessMessage("loan_repayment", "CREATE"));
      return;
    }

    const response = await fetch(form.action, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify(requestBody),
    });
    const result = await response.json();
    if (!response.ok)
      throw new Error(
        result.message || result.error || "Unable to save repayment.",
      );
    alert(result.message || "Loan repayment created successfully.");
    window.location.reload();
  } catch (error: unknown) {
    alert(error instanceof Error ? error.message : "Unable to save repayment.");
  }
}

(window as Window & typeof globalThis).openRepaymentModal = openRepaymentModal;
(window as Window & typeof globalThis).closeRepaymentModal =
  closeRepaymentModal;
(window as Window & typeof globalThis).submitRepayment = submitRepayment;
