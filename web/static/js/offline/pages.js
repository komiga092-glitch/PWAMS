import { listOfflineRecords, OFFLINE_STORES, } from "./db.js";
import { isOnline } from "./connectivity.js";
import { isOfflineSessionValid, OFFLINE_SESSION_EXPIRED_MESSAGE, } from "./session.js";
window.pwamsOfflineReadStore = listOfflineRecords;
const PAGE_CONFIGS = {
    persons: {
        store: OFFLINE_STORES.persons,
        columns: ["Full Name", "NIC / Passport", "Phone", "Status", "Actions"],
        values: (record) => [
            String(record.full_name ?? "-"),
            String(record.nic_passport ?? "-"),
            String(record.phone ?? "-"),
            String(record.status ?? "-"),
            `<a class="btn btn-small" href="/persons/${record.id}/view">View</a>`,
        ],
    },
    students: {
        store: OFFLINE_STORES.students,
        columns: [
            "Full Name",
            "School",
            "Grade",
            "Student Code",
            "Status",
            "Actions",
        ],
        values: (record) => [
            String(record.full_name ?? "-"),
            String(record.school_name ?? "-"),
            String(record.grade ?? "-"),
            String(record.student_code ?? "-"),
            String(record.status ?? "-"),
            `<a class="btn btn-small" href="/students/${record.id}/view">View</a>`,
        ],
    },
    donors: {
        store: OFFLINE_STORES.donors,
        columns: ["Name", "Phone", "Email", "Type", "Status", "Actions"],
        values: (record) => [
            String(record.name ?? "-"),
            String(record.phone ?? "-"),
            String(record.email ?? "-"),
            String(record.donor_type ?? "-"),
            String(record.status ?? "-"),
            `<a class="btn btn-small" href="/donors/${record.id}/view">View</a>`,
        ],
    },
    donations: {
        store: OFFLINE_STORES.donations,
        columns: ["Donor", "Amount", "Date", "Description", "Status"],
        values: (record) => [
            String(record.donor_id ?? "-"),
            String(record.amount ?? "-"),
            String(record.donation_date ?? "-"),
            String(record.description ?? "-"),
            String(record.status ?? "-"),
        ],
    },
    revenue: {
        store: OFFLINE_STORES.revenue,
        columns: ["Type", "Category", "Amount", "Date", "Description"],
        values: (record) => [
            String(record.record_type ?? "-"),
            String(record.category ?? "-"),
            `${String(record.currency ?? "LKR")} ${String(record.amount ?? "-")}`,
            String(record.record_date ?? "-"),
            String(record.description ?? "-"),
        ],
    },
    aid_requests: {
        store: OFFLINE_STORES.aidRequests,
        columns: ["Person", "Type", "Amount", "Date", "Status"],
        values: (record) => [
            String(record.person_id ?? "-"),
            String(record.aid_type ?? "-"),
            String(record.requested_amount ?? "-"),
            String(record.request_date ?? "-"),
            String(record.status ?? "-"),
        ],
    },
    care_provided: {
        store: OFFLINE_STORES.careProvided,
        columns: ["Person", "Care Type", "Description", "Amount", "Date", "Status"],
        values: (record) => [
            String(record.person_id ?? "-"),
            String(record.care_type ?? "-"),
            String(record.description ?? "-"),
            String(record.amount ?? "-"),
            String(record.provided_at ?? "-"),
            String(record.status ?? "-"),
        ],
    },
    loans: {
        store: OFFLINE_STORES.loans,
        columns: [
            "Person",
            "Amount",
            "Interest",
            "Duration",
            "Installment",
            "Status",
        ],
        values: (record) => [
            String(record.person_id ?? "-"),
            String(record.loan_amount ?? "-"),
            `${String(record.interest_rate ?? "-")}%`,
            `${String(record.duration_months ?? "-")} months`,
            String(record.installment_amount ?? "-"),
            String(record.status ?? "-"),
        ],
    },
    loan_repayments: {
        store: OFFLINE_STORES.loanRepayments,
        columns: ["Loan", "Amount", "Due Date", "Status"],
        values: (record) => [
            String(record.loan_id ?? "-"),
            String(record.amount ?? "-"),
            String(record.due_date ?? "-"),
            String(record.status ?? "-"),
        ],
    },
};
function escapeHTML(value) {
    const element = document.createElement("div");
    element.textContent = value;
    return element.innerHTML;
}
function setLocalIndicator() {
    const indicator = document.querySelector("[data-offline-status]");
    if (!indicator)
        return;
    indicator.textContent = "Offline - Local data";
    indicator.dataset.state = "offline";
}
function pageEntity() {
    const path = window.location.pathname;
    if (path.startsWith("/persons"))
        return "persons";
    if (path.startsWith("/students"))
        return "students";
    if (path.startsWith("/donors"))
        return "donors";
    if (path.startsWith("/donations"))
        return "donations";
    if (path.startsWith("/revenue"))
        return "revenue";
    if (path.startsWith("/aid-requests"))
        return "aid_requests";
    if (path.startsWith("/care-provided"))
        return "care_provided";
    if (path.startsWith("/loans"))
        return "loans";
    if (path.startsWith("/loan-repayments"))
        return "loan_repayments";
    return undefined;
}
function isListPage() {
    return (window.location.pathname.endsWith("/page") ||
        window.location.pathname === "/persons" ||
        window.location.pathname === "/students" ||
        window.location.pathname === "/donors");
}
async function renderOfflineList(config) {
    const records = (await listOfflineRecords(config.store)).filter((record) => !record.is_deleted);
    const body = document.querySelector("table.data-table tbody");
    if (!body)
        return;
    if (records.length === 0) {
        body.innerHTML = `<tr><td colspan="${config.columns.length}" class="empty">No offline data available.</td></tr>`;
        return;
    }
    body.innerHTML = records
        .map((record) => `<tr>${config
        .values(record)
        .map((value) => `<td>${value.startsWith("<a ") ? value : escapeHTML(value)}</td>`)
        .join("")}</tr>`)
        .join("");
}
function renderOfflineDetail(record) {
    const fields = {
        "Full Name": "full_name",
        "NIC / Passport": "nic_passport",
        "Date of Birth": "date_of_birth",
        Gender: "gender",
        Phone: "phone",
        Email: "email",
        Occupation: "occupation",
        "Monthly Income": "monthly_income",
        Address: "address",
        "Student Code": "student_code",
        "School / Institution": "school_name",
        Grade: "grade",
        "Guardian Name": "guardian_name",
        "Guardian Phone": "guardian_phone",
        "Academic Year": "academic_year",
        Remarks: "remarks",
        "Donor Type": "donor_type",
        "Organization Name": "organization_name",
        "Registration Number": "registration_number",
        "Contact Person": "contact_person_name",
        "Contact Person Phone": "contact_person_phone",
        "Preferred Donation Type": "preferred_donation_type",
        Notes: "notes",
    };
    document.querySelectorAll(".form-group").forEach((group) => {
        const label = group.querySelector("label")?.textContent?.trim();
        const key = label ? fields[label] : undefined;
        if (!key)
            return;
        const value = String(record[key] ?? "-");
        const target = group.querySelector(".form-control, input, textarea");
        if (!target)
            return;
        if (target instanceof HTMLInputElement ||
            target instanceof HTMLTextAreaElement)
            target.value = value;
        else
            target.textContent = value;
    });
}
async function renderOfflinePage() {
    if (isOnline())
        return;
    if (!(await isOfflineSessionValid())) {
        const indicator = document.querySelector("[data-offline-status]");
        if (indicator) {
            indicator.textContent = "Offline session expired";
            indicator.dataset.state = "offline";
            indicator.setAttribute("aria-label", OFFLINE_SESSION_EXPIRED_MESSAGE);
        }
        document
            .querySelectorAll("main > *:not([data-sync-conflicts])")
            .forEach((element) => {
            element.hidden = true;
        });
        document
            .querySelector("main")
            ?.insertAdjacentHTML("afterbegin", `<div class="alert alert-danger" data-offline-session-expired>${OFFLINE_SESSION_EXPIRED_MESSAGE}</div>`);
        return;
    }
    const entity = pageEntity();
    const config = entity ? PAGE_CONFIGS[entity] : undefined;
    if (!config)
        return;
    setLocalIndicator();
    if (isListPage()) {
        await renderOfflineList(config);
        return;
    }
    const parts = window.location.pathname.split("/").filter(Boolean);
    const id = parts.length > 1 ? parts[parts.length - 2] : undefined;
    if (!id)
        return;
    const records = await listOfflineRecords(config.store);
    const record = records.find((item) => item.id === id && !item.is_deleted);
    if (record)
        renderOfflineDetail(record);
    else
        document
            .querySelector("main")
            ?.insertAdjacentHTML("afterbegin", `<div class="empty">No offline data available.</div>`);
}
document.addEventListener("DOMContentLoaded", () => {
    void renderOfflinePage();
});
window.addEventListener("pwams:offline-mutation", () => {
    void renderOfflinePage();
});
//# sourceMappingURL=pages.js.map