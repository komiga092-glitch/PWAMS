"use strict";
function closeLoanModal() {
    document.getElementById("loanModal")?.classList.add("hidden");
}
function openLoanModal() {
    document.getElementById("loanModal")?.classList.remove("hidden");
}
async function loadLoanPeople() {
    const select = document.getElementById("loanPersonID");
    if (!select || select.options.length > 1)
        return;
    if (!navigator.onLine) {
        const { readOfflineStore } = await import("./offline-data.js");
        const people = await readOfflineStore("persons");
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
    if (!response.ok)
        return;
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
async function submitLoan(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = Object.fromEntries(new FormData(form));
    const requestBody = {
        ...data,
        loan_amount: Number(data.loan_amount || 0),
        interest_rate: Number(data.interest_rate || 0),
        duration_months: Number(data.duration_months || 0),
    };
    try {
        if (!navigator.onLine) {
            const { savePendingMutation, offlineSuccessMessage } = await import(String("/static/js/offline/mutations.js"));
            await savePendingMutation("loan", "CREATE", requestBody);
            alert(offlineSuccessMessage("loan", "CREATE"));
            return;
        }
        const response = await fetch(form.action, {
            method: "POST",
            credentials: "same-origin",
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
    }
    catch (error) {
        alert(error instanceof Error ? error.message : "Unable to save loan.");
    }
}
window.openLoanModal = openLoanModal;
window.closeLoanModal = closeLoanModal;
window.submitLoan = submitLoan;
//# sourceMappingURL=loans.js.map