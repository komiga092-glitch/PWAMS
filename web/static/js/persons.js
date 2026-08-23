"use strict";
const personModal = document.getElementById("person-modal");
const personForm = document.getElementById("person-create-form");
const addPersonButton = document.getElementById("add-person-btn");
const closePersonButton = document.getElementById("person-modal-close");
const cancelPersonButton = document.getElementById("person-cancel");
function openPersonModal() {
    if (!personModal) {
        return;
    }
    personModal.style.display = "flex";
    const firstInput = document.getElementById("person_full_name");
    firstInput?.focus();
}
function closePersonModal() {
    if (!personModal) {
        return;
    }
    personModal.style.display = "none";
    personForm?.reset();
}
function getPersonFormData(form) {
    const formData = new FormData(form);
    return {
        full_name: String(formData.get("full_name") ?? ""),
        nic_passport: String(formData.get("nic_passport") ?? ""),
        phone: String(formData.get("phone") ?? ""),
        gender: String(formData.get("gender") ?? ""),
        date_of_birth: String(formData.get("date_of_birth") ?? ""),
        address: String(formData.get("address") ?? ""),
        remarks: String(formData.get("remarks") ?? ""),
    };
}
async function createPerson(data) {
    const response = await fetch("/persons", {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
        },
        body: JSON.stringify(data),
    });
    const result = await response.json().catch(() => null);
    if (!response.ok) {
        throw new Error(result?.message ?? "Failed to create care seeker");
    }
    return result;
}
addPersonButton?.addEventListener("click", () => {
    openPersonModal();
});
closePersonButton?.addEventListener("click", () => {
    closePersonModal();
});
cancelPersonButton?.addEventListener("click", () => {
    closePersonModal();
});
personModal?.addEventListener("click", (event) => {
    if (event.target === personModal) {
        closePersonModal();
    }
});
personForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!personForm) {
        return;
    }
    const submitButton = personForm.querySelector('button[type="submit"]');
    try {
        if (submitButton) {
            submitButton.disabled = true;
            submitButton.textContent = "Saving...";
        }
        const data = getPersonFormData(personForm);
        await createPerson(data);
        alert("Care seeker created successfully.");
        closePersonModal();
        window.location.reload();
    }
    catch (error) {
        console.error("Person creation failed:", error);
        alert(error instanceof Error ? error.message : "Failed to create care seeker.");
    }
    finally {
        if (submitButton) {
            submitButton.disabled = false;
            submitButton.textContent = "Save Care Seeker";
        }
    }
});
//# sourceMappingURL=persons.js.map