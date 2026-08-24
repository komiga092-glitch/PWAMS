function closeAidRequestModal(): void {
  document.getElementById("aidRequestModal")?.classList.add("hidden");
  document.body.style.overflow = "";
  (
    document.getElementById("aid-request-form") as HTMLFormElement | null
  )?.reset();
}

function openAidRequestModal(): void {
  document.getElementById("aidRequestModal")?.classList.remove("hidden");
  document.body.style.overflow = "hidden";
}

async function loadAidRequestPeople(): Promise<void> {
  const select = document.getElementById(
    "personID",
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
  void loadAidRequestPeople();
});

document
  .getElementById("aidRequestModal")
  ?.addEventListener("click", (event: MouseEvent) => {
    if (event.target === event.currentTarget) closeAidRequestModal();
  });

document.addEventListener("keydown", (event: KeyboardEvent) => {
  const modal = document.getElementById("aidRequestModal");
  if (event.key === "Escape" && modal && !modal.classList.contains("hidden"))
    closeAidRequestModal();
});

async function submitAidRequest(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  const form = event.currentTarget as HTMLFormElement;
  const data = Object.fromEntries(new FormData(form)) as Record<string, string>;
  const requestBody = {
    ...data,
    requested_amount: Number(data.requested_amount || 0),
  };

  try {
    if (!navigator.onLine) {
      const { savePendingMutation, offlineSuccessMessage } = await import(
        String("/static/js/offline/mutations.js")
      );
      await savePendingMutation("aid_request", "CREATE", requestBody);
      alert(offlineSuccessMessage("aid_request", "CREATE"));
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
        result.message || result.error || "Unable to save aid request.",
      );
    alert(result.message || "Aid request created successfully.");
    window.location.reload();
  } catch (error: unknown) {
    alert(
      error instanceof Error ? error.message : "Unable to save aid request.",
    );
  }
}

(window as Window & typeof globalThis).openAidRequestModal =
  openAidRequestModal;
(window as Window & typeof globalThis).closeAidRequestModal =
  closeAidRequestModal;
(window as Window & typeof globalThis).submitAidRequest = submitAidRequest;
