"use strict";
// Loan details page (loan_details.html). The repay modal is populated from
// server-rendered data attributes and submits to the existing repayment Pay
// endpoint. All payment amounts, balances and installment values are
// recomputed server-side inside LoanRepaymentService.Pay; this script only
// renders responses, never calculates finance fields.
const PWAMS_I18N = window["PWAMS_I18N"] || {};
function repayT(key) {
    return PWAMS_I18N[key] || key;
}
function todayInputValue() {
    const now = new Date();
    const yyyy = String(now.getFullYear());
    const mm = String(now.getMonth() + 1).padStart(2, "0");
    const dd = String(now.getDate()).padStart(2, "0");
    return `${yyyy}-${mm}-${dd}`;
}
function closeRepayModal() {
    document.getElementById("repayModal")?.classList.add("hidden");
    document.body.style.overflow = "";
    const errorBox = document.getElementById("repay-error");
    if (errorBox) {
        errorBox.classList.add("hidden");
        errorBox.textContent = "";
    }
}
function showRepayError(message) {
    const errorBox = document.getElementById("repay-error");
    if (!errorBox)
        return alert(message);
    errorBox.textContent = message;
    errorBox.classList.remove("hidden");
}
function openRepayModal(button) {
    const modal = document.getElementById("repayModal");
    if (!modal)
        return;
    const setId = (id, value) => {
        const el = document.getElementById(id);
        if (el)
            el.textContent = value || "—";
    };
    document.getElementById("repayRepaymentID").value =
        button.dataset.repaymentId || "";
    setId("repayInstallment", button.dataset.installment);
    setId("repayDueDate", button.dataset.dueDate);
    setId("repayPrincipal", button.dataset.principal);
    setId("repayInterest", button.dataset.interest);
    setId("repayAmountDue", button.dataset.amountDue);
    setId("repayOutstanding", button.dataset.outstanding);
    const paymentDate = document.getElementById("repayPaymentDate");
    if (paymentDate)
        paymentDate.value = todayInputValue();
    // Default the payment amount to the server-calculated outstanding
    // amount for this installment. The server still reloads the repayment
    // record and validates the actual payable amount.
    const amountInput = document.getElementById("repayPaidAmount");
    if (amountInput && button.dataset.outstanding) {
        const parsed = Number(button.dataset.outstanding);
        amountInput.value = Number.isFinite(parsed) && parsed > 0
            ? parsed.toFixed(2)
            : "";
    }
    document.getElementById("repayPaymentMethod").value = "Cash";
    document.getElementById("repayPaymentReference").value = "";
    document.getElementById("repayNotes").value = "";
    modal.classList.remove("hidden");
    document.body.style.overflow = "hidden";
    if (amountInput)
        amountInput.focus();
}
document.addEventListener("click", (event) => {
    const button = event.target.closest("[data-repay-open]");
    if (button) {
        openRepayModal(button);
        return;
    }
    const modal = document.getElementById("repayModal");
    if (modal && event.target === modal)
        closeRepayModal();
});
document.addEventListener("keydown", (event) => {
    const modal = document.getElementById("repayModal");
    if (event.key === "Escape" && modal && !modal.classList.contains("hidden"))
        closeRepayModal();
});
// statusBadge renders the localized HTML badge for one repayment status.
function statusBadge(status) {
    const key = status === "Paid"
        ? "status.paid"
        : status === "Partially Paid"
            ? "status.partially_paid"
            : status === "Overdue"
                ? "status.overdue"
                : status === "Cancelled"
                    ? "status.cancelled"
                    : "status.pending";
    const className = status === "Paid"
        ? "badge badge-success"
        : status === "Partially Paid"
            ? "badge badge-warning"
            : status === "Overdue"
                ? "badge badge-danger"
                : status === "Cancelled"
                    ? "badge badge-muted"
                    : "badge badge-warning";
    return `<span class="${className}">${repayT(key)}</span>`;
}
// repayButtonHTML rebuilds the repay action for one row after a payment.
function repayButtonHTML(installment) {
    if (!installment)
        return "";
    const canRepay = (window.PWAMS_PERMISSIONS || {})["repayment.create"];
    const payable = installment.status === "Pending" ||
        installment.status === "Partially Paid" ||
        installment.status === "Overdue";
    const outstanding = Number(installment.outstanding_amount || 0);
    if (!canRepay || !payable || outstanding <= 0)
        return "";
    const money = (value) => Number(value || 0).toFixed(2);
    const due = installment.due_date ? String(installment.due_date).slice(0, 10) : "";
    return `<button type="button" class="btn btn-small btn-primary" data-repay-open
        data-repayment-id="${installment.id}"
        data-installment="${installment.installment_number}"
        data-due-date="${due}"
        data-principal="${money(installment.principal_amount)}"
        data-interest="${money(installment.interest_amount)}"
        data-amount-due="${money(installment.amount)}"
        data-outstanding="${outstanding.toFixed(2)}">${repayT("loan.repay")}</button>`;
}
// renderScheduleRow builds one table row from an authoritative server
// repayment object (returned by GET /loan-repayments?loan_id=...). The
// script never computes balance figures; it only formats server values.
function renderScheduleRow(repayment) {
    const due = repayment.due_date ? String(repayment.due_date).slice(0, 10) : "—";
    const money = (value) => Number(value || 0).toFixed(2);
    const action = repayButtonHTML(repayment) ||
        (repayment.status === "Paid"
            ? `<span class="muted">${repayT("status.paid")}</span>`
            : "");
    return `<tr data-repayment-row data-repayment-id="${repayment.id}" data-row-status="${repayment.status}">
        <td>#${repayment.installment_number}</td>
        <td>${due}</td>
        <td>${money(repayment.principal_amount)}</td>
        <td>${money(repayment.interest_amount)}</td>
        <td>${money(repayment.amount)}</td>
        <td data-row-paid>${money(repayment.paid_amount)}</td>
        <td data-row-outstanding>${money(repayment.outstanding_amount)}</td>
        <td>${statusBadge(repayment.status)}</td>
        <td class="actions">${action}</td>
      </tr>`;
}
// refreshSchedule rebuilds the schedule table body from the server's current
// repayment list so paid amounts, outstanding balances and statuses are all
// server-authoritative after a payment.
function refreshSchedule(repayments) {
    const body = document.querySelector("[data-schedule-body]");
    if (!body)
        return false;
    if (!repayments || repayments.length === 0) {
        body.innerHTML = `<tr><td colspan="9" class="empty">${repayT("loan.no_repayments")}</td></tr>`;
        return true;
    }
    body.innerHTML = repayments
        .sort((a, b) => a.installment_number - b.installment_number)
        .map((r) => renderScheduleRow(r))
        .join("");
    return true;
}
// setSummaryCard writes a value into one labelled financial-summary card.
function setSummaryCard(attr, value) {
    const el = document.querySelector(`[data-summary="${attr}"]`);
    if (el)
        el.textContent = value || "—";
}
// refreshLoanSummary reflects the fresh loan totals supplied by the finder.
function refreshLoanSummary(loan) {
    setSummaryCard("total-paid", Number(loan.total_paid_amount || 0).toFixed(2));
    setSummaryCard("outstanding", Number(loan.outstanding_amount || 0).toFixed(2));
    if (loan.status === "Completed") {
        const badge = document.querySelector("[data-loan-status-badge]");
        if (badge) {
            badge.className = "badge badge-success";
            badge.textContent = repayT("loan.fully_paid");
        }
        const statusCard = document.querySelector("[data-loan-status-value]");
        if (statusCard)
            statusCard.textContent = repayT("loan.fully_paid");
    }
}
// refreshNextPayment picks the first still-payable installment from the fresh
// schedule and reflects it in the Next Payment / Next Due Date cards.
function refreshNextPayment(repayments) {
    const next = (repayments || [])
        .filter((r) => ["Pending", "Partially Paid", "Overdue"].includes(r.status))
        .sort((a, b) => a.installment_number - b.installment_number)[0];
    if (next) {
        setSummaryCard("next-payment", Number(next.outstanding_amount || 0).toFixed(2));
        const due = next.due_date ? String(next.due_date).slice(0, 10) : "—";
        setSummaryCard("next-due-date", due);
    }
    else {
        setSummaryCard("next-payment", "—");
        setSummaryCard("next-due-date", "—");
    }
}
// refreshLoanAfterPayment re-reads the authoritative loan and repayment list
// from the server and updates the page in place (no full page reload).
async function refreshLoanAfterPayment(loanID) {
    const loanResponse = await fetch(`/loans/${loanID}`, {
        credentials: "same-origin",
        headers: { Accept: "application/json" },
    });
    if (loanResponse.ok) {
        const loanResult = await loanResponse.json();
        const loan = loanResult.loan;
        if (loan)
            refreshLoanSummary(loan);
    }
    const repaymentsResponse = await fetch(`/loan-repayments?loan_id=${encodeURIComponent(loanID)}`, {
        credentials: "same-origin",
        headers: { Accept: "application/json" },
    });
    if (repaymentsResponse.ok) {
        const result = await repaymentsResponse.json();
        const repayments = result.data && result.data.repayments
            ? result.data.repayments
            : [];
        refreshSchedule(repayments);
        refreshNextPayment(repayments);
    }
}
// showSuccessMessage shows a localized success banner at the top of the page.
function showSuccessMessage(message) {
    const page = document.querySelector(".page");
    if (!page)
        return alert(message);
    const banner = document.createElement("div");
    banner.className = "alert alert-success";
    banner.textContent = message;
    page.prepend(banner);
}
async function submitRepaymentPayment(event) {
    event.preventDefault();
    const form = document.getElementById("repay-form");
    const repaymentID = document.getElementById("repayRepaymentID").value;
    if (!repaymentID)
        return;
    const submitButton = form.querySelector('button[type="submit"]');
    if (submitButton)
        submitButton.disabled = true;
    showRepayError("");
    const payload = {
        paid_amount: Number(document.getElementById("repayPaidAmount").value || 0),
        payment_method: document.getElementById("repayPaymentMethod").value || "",
        payment_reference: document.getElementById("repayPaymentReference").value || "",
        notes: document.getElementById("repayNotes").value || "",
    };
    try {
        const response = await fetch(`/loan-repayments/${repaymentID}/pay`, {
            method: "PATCH",
            credentials: "same-origin",
            headers: {
                "Content-Type": "application/json",
                Accept: "application/json",
            },
            body: JSON.stringify(payload),
        });
        const result = await response.json();
        if (!response.ok)
            throw new Error(result.message || repayT("loan.payment_error"));
        const repayment = result.repayment;
        const loanID = repayment ? repayment.loan_id || "" : "";
        closeRepayModal();
        showSuccessMessage(repayT("loan.payment_successful"));
        if (loanID) {
            try {
                await refreshLoanAfterPayment(loanID);
                return;
            }
            catch (error) {
                // Partial refresh failed; fall back to a full reload so the
                // displayed figures are still refreshed from the server.
            }
        }
        window.location.reload();
    }
    catch (error) {
        showRepayError(error instanceof Error ? error.message : repayT("loan.payment_error"));
    }
    finally {
        if (submitButton)
            submitButton.disabled = false;
    }
}
document.addEventListener("DOMContentLoaded", () => {
    const modal = document.getElementById("repayModal");
    if (modal) {
        modal.addEventListener("click", (event) => {
            if (event.target === modal)
                closeRepayModal();
        });
    }
});
window.openRepayModal = openRepayModal;
window.closeRepayModal = closeRepayModal;
window.submitRepaymentPayment = submitRepaymentPayment;
//# sourceMappingURL=loan-details.js.map