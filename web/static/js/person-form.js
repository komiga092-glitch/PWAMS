"use strict";
const standalonePersonForm = document.getElementById("person-form");
standalonePersonForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const values = Object.fromEntries(new FormData(form));
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
            const { savePendingMutation, offlineSuccessMessage } = await import(String("/static/js/offline/mutations.js"));
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
        const result = (await response.json());
        if (!response.ok || result.success === false) {
            throw new Error(result.message || result.error || "Unable to create person.");
        }
        alert(result.message || "Person created successfully.");
        window.location.href = "/persons/page";
    }
    catch (error) {
        alert(error instanceof Error ? error.message : "Unable to create person.");
    }
});
//# sourceMappingURL=person-form.js.map