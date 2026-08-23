interface UpdatePersonRequest {
  full_name: string;
  nic_passport: string;
  date_of_birth: string;
  gender: string;
  phone: string;
  email: string;
  address: string;
  occupation: string;
  monthly_income: number;
  status: string;
}

interface PersonUpdateResponse {
  success: boolean;
  message?: string;
}

const personEditForm = document.getElementById(
  "person-edit-form",
) as HTMLFormElement | null;

async function updatePerson(
  id: string,
  data: UpdatePersonRequest,
): Promise<PersonUpdateResponse> {
  const response = await fetch(`/persons/${id}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
    },
    body: JSON.stringify(data),
  });

  const result = (await response.json().catch(() => null)) as
    | PersonUpdateResponse
    | { message?: string }
    | null;

  if (!response.ok) {
    throw new Error(
      result && "message" in result && result.message
        ? result.message
        : "Failed to update care seeker",
    );
  }

  return result as PersonUpdateResponse;
}

personEditForm?.addEventListener("submit", async (event: SubmitEvent) => {
  event.preventDefault();

  const personID = personEditForm.dataset.personId;

  if (!personID) {
    alert("Invalid person ID.");
    return;
  }

  const formData = new FormData(personEditForm);

  const data: UpdatePersonRequest = {
    full_name: String(formData.get("full_name") ?? ""),

    nic_passport: String(formData.get("nic_passport") ?? ""),

    date_of_birth: String(formData.get("date_of_birth") ?? ""),

    gender: String(formData.get("gender") ?? ""),

    phone: String(formData.get("phone") ?? ""),

    email: String(formData.get("email") ?? ""),

    address: String(formData.get("address") ?? ""),

    occupation: String(formData.get("occupation") ?? ""),

    monthly_income: Number(formData.get("monthly_income") ?? 0),

    status: String(formData.get("status") ?? ""),
  };

  const submitButton = personEditForm.querySelector(
    'button[type="submit"]',
  ) as HTMLButtonElement | null;

  try {
    if (submitButton) {
      submitButton.disabled = true;
      submitButton.textContent = "Updating...";
    }

    await updatePerson(personID, data);

    alert("Care seeker updated successfully.");

    window.location.href = `/persons/${personID}/view`;
  } catch (error) {
    console.error("Person update failed:", error);

    alert(
      error instanceof Error ? error.message : "Failed to update care seeker.",
    );
  } finally {
    if (submitButton) {
      submitButton.disabled = false;
      submitButton.textContent = "Update Care Seeker";
    }
  }
});
