interface UpdateDonorRequest {
  name: string;
  donor_type: string;
  nic_passport: string;
  organization_name: string;
  registration_number: string;
  phone: string;
  email: string;
  address: string;
  contact_person_name: string;
  contact_person_phone: string;
  preferred_donation_type: string;
  notes: string;
  status: string;
}

const donorEditForm = document.getElementById(
  "donor-edit-form",
) as HTMLFormElement | null;

donorEditForm?.addEventListener("submit", async (event: SubmitEvent) => {
  event.preventDefault();

  const donorID = donorEditForm.dataset.donorId?.trim();

  if (!donorID) {
    alert("Invalid donor ID.");
    return;
  }

  const formData = new FormData(donorEditForm);

  // =========================
  // NORMALIZE VALUES
  // =========================

  const donorType = String(formData.get("donor_type") ?? "")
    .trim()
    .toLowerCase();

  const status = String(formData.get("status") ?? "").trim();

  // =========================
  // VALIDATION
  // =========================

  if (!String(formData.get("name") ?? "").trim()) {
    alert("Donor name is required.");
    return;
  }

  if (donorType !== "individual" && donorType !== "organization") {
    alert("Please select a valid donor type.");
    return;
  }

  // =========================
  // REQUEST DATA
  // =========================

  const data: UpdateDonorRequest = {
    name: String(formData.get("name") ?? "").trim(),

    donor_type: donorType,

    nic_passport: String(formData.get("nic_passport") ?? "").trim(),

    organization_name: String(formData.get("organization_name") ?? "").trim(),

    registration_number: String(
      formData.get("registration_number") ?? "",
    ).trim(),

    phone: String(formData.get("phone") ?? "").trim(),

    email: String(formData.get("email") ?? "").trim(),

    address: String(formData.get("address") ?? "").trim(),

    contact_person_name: String(
      formData.get("contact_person_name") ?? "",
    ).trim(),

    contact_person_phone: String(
      formData.get("contact_person_phone") ?? "",
    ).trim(),

    preferred_donation_type: String(
      formData.get("preferred_donation_type") ?? "",
    ).trim(),

    notes: String(formData.get("notes") ?? "").trim(),

    status: status,
  };

  // =========================
  // UPDATE DONOR
  // =========================

  const updateButton = document.getElementById(
    "donor-update-btn",
  ) as HTMLButtonElement | null;

  try {
    if (updateButton) {
      updateButton.disabled = true;
      updateButton.textContent = "Updating...";
    }

    if (!navigator.onLine) {
      const { savePendingMutation, offlineSuccessMessage } = await import(
        String("/static/js/offline/mutations.js")
      );
      await savePendingMutation(
        "donor",
        "UPDATE",
        data as unknown as Record<string, unknown>,
        donorID,
      );
      alert(offlineSuccessMessage("donor", "UPDATE"));
      return;
    }

    const response = await fetch(`/donors/${donorID}`, {
      method: "PUT",

      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },

      body: JSON.stringify(data),
    });

    // =========================
    // SERVER RESPONSE
    // =========================

    let result: {
      success?: boolean;
      message?: string;
      error?: string;
    };

    try {
      result = await response.json();
    } catch {
      throw new Error("Invalid server response.");
    }

    if (!response.ok) {
      throw new Error(
        result.message ?? result.error ?? "Failed to update donor.",
      );
    }

    // =========================
    // SUCCESS
    // =========================

    alert("Donor updated successfully.");

    window.location.href = `/donors/${donorID}/view`;
  } catch (error: unknown) {
    console.error("Donor update failed:", error);

    alert(error instanceof Error ? error.message : "Failed to update donor.");
  } finally {
    if (updateButton) {
      updateButton.disabled = false;
      updateButton.textContent = "Update Donor";
    }
  }
});
