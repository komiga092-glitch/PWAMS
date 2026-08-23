interface CreateDonorRequest {
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
}

interface DonorResponse {
  success: boolean;
  message?: string;
  error?: string;
  donor?: unknown;
}


/* =========================
   DOM ELEMENTS
   ========================= */

const donorModal =
  document.getElementById("donorModal") as HTMLDivElement | null;

const donorForm =
  document.getElementById(
    "donor-create-form",
  ) as HTMLFormElement | null;

const addDonorButton =
  document.getElementById("add-donor-btn");

const closeDonorButton =
  document.getElementById("donor-modal-close");

const cancelDonorButton =
  document.getElementById("donor-cancel");

const donorSaveButton =
  document.getElementById("donor-save-btn") as HTMLButtonElement | null;


/* =========================
   OPEN MODAL
   ========================= */

function openDonorModal(): void {
  if (!donorModal) {
    console.error("Donor modal not found.");
    return;
  }

  donorModal.style.display = "flex";

  document.body.style.overflow = "hidden";
}


/* =========================
   CLOSE MODAL
   ========================= */

function closeDonorModal(): void {
  if (!donorModal) {
    return;
  }

  donorModal.style.display = "none";

  document.body.style.overflow = "";
}


/* =========================
   BUTTON EVENTS
   ========================= */

addDonorButton?.addEventListener(
  "click",
  openDonorModal,
);

closeDonorButton?.addEventListener(
  "click",
  closeDonorModal,
);

cancelDonorButton?.addEventListener(
  "click",
  closeDonorModal,
);


/* =========================
   CLICK OUTSIDE MODAL
   ========================= */

donorModal?.addEventListener(
  "click",
  (event: MouseEvent) => {

    if (event.target === donorModal) {
      closeDonorModal();
    }

  },
);


/* =========================
   ESC KEY
   ========================= */

document.addEventListener(
  "keydown",
  (event: KeyboardEvent) => {

    if (event.key === "Escape") {

      if (
        donorModal &&
        donorModal.style.display === "flex"
      ) {
        closeDonorModal();
      }

    }

  },
);


/* =========================
   FORM DATA
   ========================= */

function getDonorFormData(
  form: HTMLFormElement,
): CreateDonorRequest {

  const formData =
    new FormData(form);

  return {

    name: String(
      formData.get("name") ?? "",
    ),

    donor_type: String(
      formData.get("donor_type") ?? "",
    ),

    nic_passport: String(
      formData.get("nic_passport") ?? "",
    ),

    organization_name: String(
      formData.get("organization_name") ?? "",
    ),

    registration_number: String(
      formData.get("registration_number") ?? "",
    ),

    phone: String(
      formData.get("phone") ?? "",
    ),

    email: String(
      formData.get("email") ?? "",
    ),

    address: String(
      formData.get("address") ?? "",
    ),

    contact_person_name: String(
      formData.get("contact_person_name") ?? "",
    ),

    contact_person_phone: String(
      formData.get("contact_person_phone") ?? "",
    ),

    preferred_donation_type: String(
      formData.get(
        "preferred_donation_type",
      ) ?? "",
    ),

    notes: String(
      formData.get("notes") ?? "",
    ),

  };
}


/* =========================
   CREATE DONOR
   ========================= */

async function createDonor(
  data: CreateDonorRequest,
): Promise<DonorResponse> {

  const response =
    await fetch(
      "/donors",
      {
        method: "POST",

        headers: {
          "Content-Type":
            "application/json",

          Accept:
            "application/json",
        },

        body:
          JSON.stringify(data),
      },
    );


  let result: DonorResponse;

  try {

    result =
      await response.json();

  } catch {

    throw new Error(
      "Invalid server response.",
    );

  }


  if (!response.ok) {

    throw new Error(
      result.message ??
      result.error ??
      "Failed to create donor.",
    );

  }


  return result;
}


/* =========================
   FORM SUBMIT
   ========================= */

donorForm?.addEventListener(
  "submit",
  async (event: SubmitEvent) => {

    event.preventDefault();


    const data =
      getDonorFormData(donorForm);


    if (!data.name.trim()) {

      alert(
        "Donor name is required.",
      );

      return;

    }


    try {

      if (donorSaveButton) {

        donorSaveButton.disabled =
          true;

        donorSaveButton.textContent =
          "Saving...";

      }


      await createDonor(data);


      alert(
        "Donor registered successfully.",
      );


      donorForm.reset();

      closeDonorModal();


      window.location.reload();


    } catch (error) {

      console.error(
        "Donor creation failed:",
        error,
      );


      alert(
        error instanceof Error
          ? error.message
          : "Failed to create donor.",
      );


    } finally {

      if (donorSaveButton) {

        donorSaveButton.disabled =
          false;

        donorSaveButton.textContent =
          "Save Donor";

      }

    }

  },
);