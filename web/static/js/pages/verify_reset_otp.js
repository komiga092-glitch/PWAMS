
      const params = new URLSearchParams(window.location.search);
      const email = params.get("email");

      const emailInput = document.getElementById("email");
      const form = document.getElementById("verifyOtpForm");
      const message = document.getElementById("message");

      if (!email) {
        message.style.display = "block";
        message.textContent = "Email address is missing.";
        form.style.display = "none";
      } else {
        emailInput.value = email;
      }

      form.addEventListener("submit", async function (event) {
        event.preventDefault();

        const otp = document.getElementById("otp").value.trim();

        message.style.display = "none";

        try {
          const response = await fetch("/verify-reset-otp", {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify({
              email: email,
              otp: otp,
            }),
          });

          const data = await response.json();

          message.style.display = "block";
          message.textContent = data.message || "Request completed.";

          if (response.ok && data.success) {
            window.location.href =
              "/reset-password?email=" +
              encodeURIComponent(email) +
              "&otp=" +
              encodeURIComponent(otp);
          }
        } catch (error) {
          message.style.display = "block";
          message.textContent = "Unable to verify OTP. Please try again.";
        }
      });
    
