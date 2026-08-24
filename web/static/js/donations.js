"use strict";
function closeDonationModal() {
    document.getElementById("donationModal")?.classList.add("hidden");
    document.body.style.overflow = "";
    document.getElementById("donation-form")?.reset();
}
function openDonationModal() {
    document.getElementById("donationModal")?.classList.remove("hidden");
    document.body.style.overflow = "hidden";
}
document
    .getElementById("donationModal")
    ?.addEventListener("click", (event) => {
    if (event.target === event.currentTarget)
        closeDonationModal();
});
document.addEventListener("keydown", (event) => {
    const modal = document.getElementById("donationModal");
    if (event.key === "Escape" && modal && !modal.classList.contains("hidden"))
        closeDonationModal();
});
async function submitDonation(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = Object.fromEntries(new FormData(form));
    data.amount = String(Number(data.amount || 0));
    data.quantity = String(Number(data.quantity || 0));
    try {
        if (!navigator.onLine) {
            const { savePendingMutation, offlineSuccessMessage } = await import(String("/static/js/offline/mutations.js"));
            await savePendingMutation("donation", "CREATE", {
                ...data,
                amount: Number(data.amount),
                quantity: Number(data.quantity),
            });
            alert(offlineSuccessMessage("donation", "CREATE"));
            return;
        }
        const response = await fetch(form.action, {
            method: "POST",
            credentials: "include",
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
            throw new Error(result.message || result.error || "Unable to save donation.");
        alert(result.message || "Donation registered successfully.");
        window.location.reload();
    }
    catch (error) {
        alert(error instanceof Error ? error.message : "Unable to save donation.");
    }
}
window.openDonationModal = openDonationModal;
window.closeDonationModal = closeDonationModal;
window.submitDonation = submitDonation;
//# sourceMappingURL=donations.js.map