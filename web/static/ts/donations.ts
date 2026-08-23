type FormSubmitEvent = SubmitEvent & { currentTarget: HTMLFormElement };

function closeDonationModal(): void {
  document.getElementById("donationModal")?.classList.add("hidden");
}

function openDonationModal(): void {
  document.getElementById("donationModal")?.classList.remove("hidden");
}

async function submitDonation(event: SubmitEvent): Promise<void> {
  event.preventDefault();
  const form = event.currentTarget as HTMLFormElement;
  const data = Object.fromEntries(new FormData(form)) as Record<string, string>;
  data.amount = String(Number(data.amount || 0));
  data.quantity = String(Number(data.quantity || 0));

  try {
    const response = await fetch(form.action, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify({
        ...data,
        amount: Number(data.amount),
        quantity: Number(data.quantity),
      }),
    });
    const result = await response.json();
    if (!response.ok)
      throw new Error(
        result.message || result.error || "Unable to save donation.",
      );
    alert(result.message || "Donation registered successfully.");
    window.location.reload();
  } catch (error: unknown) {
    alert(error instanceof Error ? error.message : "Unable to save donation.");
  }
}

(window as Window & typeof globalThis).openDonationModal = openDonationModal;
(window as Window & typeof globalThis).closeDonationModal = closeDonationModal;
(window as Window & typeof globalThis).submitDonation = submitDonation;
