"use strict";
const personEditForm = document.getElementById("person-edit-form");
async function updatePerson(id, data) {
    if (!navigator.onLine) {
        const { savePendingMutation, offlineSuccessMessage } = await import(String("/static/js/offline/mutations.js"));
        await savePendingMutation("person", "UPDATE", data, id);
        return {
            success: true,
            message: offlineSuccessMessage("person", "UPDATE"),
        };
    }
    const response = await fetch(`/persons/${id}`, {
        method: "PUT",
        credentials: "include",
        headers: {
            "Content-Type": "application/json",
            Accept: "application/json",
        },
        body: JSON.stringify(data),
    });
    const result = (await response.json().catch(() => null));
    if (!response.ok ||
        (result && "success" in result && result.success === false)) {
        throw new Error(result && "message" in result && result.message
            ? result.message
            : "Failed to update care seeker");
    }
    return result;
}
personEditForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const personID = personEditForm.dataset.personId;
    if (!personID) {
        alert("Invalid person ID.");
        return;
    }
    const formData = new FormData(personEditForm);
    const data = {
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
    const submitButton = personEditForm.querySelector('button[type="submit"]');
    try {
        if (submitButton) {
            submitButton.disabled = true;
            submitButton.textContent = "Updating...";
        }
        await updatePerson(personID, data);
        alert("Care seeker updated successfully.");
        if (!navigator.onLine)
            return;
        window.location.href = `/persons/${personID}/view`;
    }
    catch (error) {
        console.error("Person update failed:", error);
        alert(error instanceof Error ? error.message : "Failed to update care seeker.");
    }
    finally {
        if (submitButton) {
            submitButton.disabled = false;
            submitButton.textContent = "Update Care Seeker";
        }
    }
});
//# sourceMappingURL=person-edit.js.map