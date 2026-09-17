
  async function submitRepaymentAction(event, form, action) {
    event.preventDefault();
    const data = Object.fromEntries(new FormData(form));
    if (action === "pay") data.paid_amount = Number(data.paid_amount);
    try {
      const response = await fetch(form.action, {
        method: "PATCH",
        credentials: "same-origin",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: action === "pay" ? JSON.stringify(data) : "{}",
      });
      const result = await response.json();
      if (!response.ok)
        throw new Error(result.message || "Unable to update repayment.");
      alert(result.message || "Repayment updated successfully.");
      window.location.reload();
    } catch (error) {
      alert(
        error instanceof Error ? error.message : "Unable to update repayment.",
      );
    }
  }

