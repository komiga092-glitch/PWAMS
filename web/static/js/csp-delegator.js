// csp-delegator.js
// ---------------------------------------------------------------------------
// Central event delegation for CSP-compliant UI actions.
//
// Every inline onclick/onsubmit handler attribute that lived inside the
// templates was replaced with a `data-csp-action` attribute (plus optional
// `data-csp-*` argument attributes). Inline <script> blocks were moved to
// external page scripts under /static/js/pages/. This file dispatches the
// data-csp-action values to the corresponding global page functions.
//
// The application serves a strict Content-Security-Policy
// (script-src 'self') and this file must NOT be weakened. Everything wired
// here lives in external scripts, so inline execution stays blocked.
(function () {
  "use strict";

  var CLICK = {
    // Users page (web/static/js/pages/users.js)
    "users:open": function () { openUserModal(); },
    "users:close": function () { closeUserModal(); },
    "users:close-password": function () { closePasswordModal(); },
    "users:edit": function (el) { editUser(el.dataset.cspId); },
    "users:change-password": function (el) { changeUserPassword(el.dataset.cspId); },
    "users:change-status": function (el) { changeUserStatus(el.dataset.cspId, el.dataset.cspStatus); },
    "users:delete": function (el) { deleteUser(el.dataset.cspId, el.dataset.cspName); },
    "users:page": function (el) { loadUsers(Number(el.dataset.cspPage)); },

    // Care provided page (web/static/js/pages/care_provided.js)
    "care:open": function () { openCareModal(); },
    "care:close": function () { closeCareModal(); },
    "care:prev": function () { previousPage(); },
    "care:next": function () { nextPage(); },
    "care:view": function (el) { viewCareRecord(el.dataset.cspId); },
    "care:edit": function (el) { editCareRecord(el.dataset.cspId); },
    "care:delete": function (el) { deleteCareRecord(el.dataset.cspId); },

    // Aid requests page (web/static/js/aid-requests.js)
    "aid:open": function () { openAidRequestModal(); },
    "aid:close": function () { closeAidRequestModal(); },

    // Donations page (web/static/js/donations.js)
    "donations:open": function () { openDonationModal(); },
    "donations:close": function () { closeDonationModal(); },

    // Loans page (web/static/js/loans.js)
    "loans:open": function () { openLoanModal(); },
    "loans:close": function () { closeLoanModal(); },

    // Loan repayments page (web/static/js/loan-repayments.js)
    "repay:open": function () { openRepaymentModal(); },
    "repay:close": function () { closeRepaymentModal(); },

    // Revenue page (web/static/js/revenue.js)
    "revenue:open": function () { openRevenueModal(); },
    "revenue:close": function () { closeRevenueModal(); },
    "revenue:delete": function (el) { deleteRevenueRecord(el.dataset.cspId); },
  };

  var SUBMIT = {
    "aid:submit": function (event) { submitAidRequest(event); },
    "care:submit": function (event) { handleCareFormSubmit(event); },
    "care:search": function (event) {
      event.preventDefault();
      loadCareRecords(1);
      return false;
    },
    "donations:submit": function (event) { submitDonation(event); },
    "loans:submit": function (event) { submitLoan(event); },
    "repay:pay": function (event, form) { submitRepaymentAction(event, form, "pay"); },
    "repay:cancel": function (event, form) { submitRepaymentAction(event, form, "cancel"); },
    "repay:submit": function (event) { submitRepayment(event); },
    "revenue:submit": function (event) { submitRevenue(event); },
  };

  document.addEventListener("click", function (event) {
    var target = event.target;
    if (!target || typeof target.closest !== "function") return;
    var el = target.closest("[data-csp-action]");
    if (!el) return;
    var handler = CLICK[el.dataset.cspAction];
    if (!handler) return;
    handler(el, event);
  });

  document.addEventListener("submit", function (event) {
    var target = event.target;
    if (!target || typeof target.closest !== "function") return;
    var el = target.closest("form[data-csp-action]");
    if (!el) return;
    var handler = SUBMIT[el.dataset.cspAction];
    if (!handler) return;
    handler(event, el);
  });
})();