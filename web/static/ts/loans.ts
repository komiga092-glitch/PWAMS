function closeLoanModal(): void {
  document.getElementById("loanModal")?.classList.add("hidden");
  document.body.style.overflow = "";
  (document.getElementById("loan-form") as HTMLFormElement | null)?.reset();
}

function openLoanModal(): void {
  document.getElementById("loanModal")?.classList.remove("hidden");
  document.body.style.overflow = "hidden";
}

document
  .getElementById("loanModal")
  ?.addEventListener("click", (event: MouseEvent) => {
    if (event.target === event.currentTarget) closeLoanModal();
  });

document.addEventListener("keydown", (event: KeyboardEvent) => {
  const modal = document.getElementById("loanModal");
  if (event.key === "Escape" && modal && !modal.classList.contains("hidden"))
    closeLoanModal();
});

async function loadLoanPeople(): Promise<void> {
  const select = document.getElementById(
    "loanPersonID",
  ) as HTMLSelectElement | null;
  if (!select || select.options.length > 1) return;

  if (!navigator.onLine) {
    const { readOfflineStore } = await import("./offline-data.js");
    const people = await readOfflineStore<{
      id: string;
      full_name: string;
      nic_passport: string;
      is_deleted?: boolean;
    }>("persons");
    for (const person of people.filter((item) => !item.is_deleted)) {
      const option = document.createElement("option");
      option.value = person.id;
      option.textContent = `${person.full_name} (${person.nic_passport})`;
      select.appendChild(option);
    }
    return;
  }

  const response = await fetch("/persons?page_size=100", {
    credentials: "same-origin",
  });
  if (!response.ok) return;
  const result = await response.json();
  for (const person of result.data || []) {
    const option = document.createElement("option");
    option.value = person.id;
    option.textContent = `${person.full_name} (${person.nic_passport})`;
    select.appendChild(option);
  }
}

document.addEventListener("DOMContentLoaded", () => {
  void loadLoanPeople();
});

async function submitLoan(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  const form = event.currentTarget as HTMLFormElement;
  const submitButton = form.querySelector<HTMLButtonElement>(
    'button[type="submit"]',
  );
  if (submitButton) submitButton.disabled = true;
  const data = Object.fromEntries(new FormData(form)) as Record<string, string>;
  const requestBody = {
    ...data,
    loan_amount: Number(data.loan_amount || 0),
    interest_rate: Number(data.interest_rate || 0),
    duration_months: Number(data.duration_months || 0),
  };

  try {
    if (!navigator.onLine) {
      const { savePendingMutation, offlineSuccessMessage } = await import(
        String("/static/js/offline/mutations.js")
      );
      await savePendingMutation("loan", "CREATE", requestBody);
      alert(offlineSuccessMessage("loan", "CREATE"));
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
      throw new Error(result.message || result.error || "Unable to save loan.");
    alert(result.message || "Loan created successfully.");
    window.location.reload();
  } catch (error: unknown) {
    alert(error instanceof Error ? error.message : "Unable to save loan.");
  } finally {
    if (submitButton) submitButton.disabled = false;
  }
}

(window as Window & typeof globalThis).openLoanModal = openLoanModal;
(window as Window & typeof globalThis).closeLoanModal = closeLoanModal;
(window as Window & typeof globalThis).submitLoan = submitLoan;
