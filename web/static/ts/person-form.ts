interface StandalonePersonResponse {
  success?: boolean;
  message?: string;
  error?: string;
}

const standalonePersonForm = document.getElementById(
  "person-form",
) as HTMLFormElement | null;

standalonePersonForm?.addEventListener("submit", async (event: SubmitEvent) => {
  event.preventDefault();
  const form = event.currentTarget as HTMLFormElement;
  const values = Object.fromEntries(new FormData(form)) as Record<
    string,
    string
  >;
  const requestBody = {
    full_name: values.full_name?.trim() ?? "",
    nic_passport: values.nic_passport?.trim() ?? "",
    date_of_birth: values.date_of_birth ?? "",
    gender: values.gender ?? "",
    phone: values.phone?.trim() ?? "",
    email: values.email?.trim() ?? "",
    address: values.address?.trim() ?? "",
    occupation: values.occupation?.trim() ?? "",
    monthly_income: Number(values.monthly_income || 0),
  };

  try {
    if (!navigator.onLine) {
      const { savePendingMutation, offlineSuccessMessage } = await import(
        String("/static/js/offline/mutations.js")
      );
      await savePendingMutation("person", "CREATE", requestBody);
      alert(offlineSuccessMessage("person", "CREATE"));
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
    const result = (await response.json()) as StandalonePersonResponse;
    if (!response.ok || result.success === false) {
      throw new Error(
        result.message || result.error || "Unable to create person.",
      );
    }
    alert(result.message || "Person created successfully.");
    window.location.href = "/persons/page";
  } catch (error: unknown) {
    alert(error instanceof Error ? error.message : "Unable to create person.");
  }
});
