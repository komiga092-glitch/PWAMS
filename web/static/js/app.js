function setupJSONForm(formSelector, options = {}) {
  const form = document.querySelector(formSelector);

  if (!form) return;

  form.addEventListener("submit", async (event) => {
    event.preventDefault();

    const data = Object.fromEntries(new FormData(form).entries());

    if (options.transform) {
      options.transform(data);
    }

    try {
      const response = await fetch(form.action, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify(data),
      });

      const result = await response.json();

      if (!response.ok || !result.success) {
        throw new Error(result.message || result.error || "Request failed");
      }

      if (options.onSuccess) {
        options.onSuccess(result);
      } else {
        window.location.reload();
      }
    } catch (error) {
      alert(error.message);
    }
  });
}

/* =========================
   Care Provided
   ========================= */

setupJSONForm('form[action="/care-provided"]', {
  transform(data) {
    data.amount = Number(data.amount || 0);
  },
});

/* =========================
   Donations
   ========================= */

setupJSONForm('form[action="/donations"]', {
  transform(data) {
    data.amount = Number(data.amount || 0);
    data.quantity = Number(data.quantity || 0);
  },
});

/* =========================
   Aid Requests
   ========================= */

setupJSONForm('form[action="/aid-requests"]', {
  transform(data) {
    data.requested_amount = Number(data.requested_amount || 0);
  },
});

/* =========================
   Loans
   ========================= */

setupJSONForm('form[action="/loans"]', {
  transform(data) {
    data.loan_amount = Number(data.loan_amount || 0);

    data.interest_rate = Number(data.interest_rate || 0);

    data.duration_months = Number(data.duration_months || 0);
  },
});

/* =========================
   Loan Repayments
   ========================= */

setupJSONForm('form[action="/loan-repayments"]', {
  transform(data) {
    data.installment_number = Number(data.installment_number || 1);

    data.amount = Number(data.amount || 0);

    if (data.payment_date) {
      data.due_date = data.payment_date;
      delete data.payment_date;
    }
  },
});
