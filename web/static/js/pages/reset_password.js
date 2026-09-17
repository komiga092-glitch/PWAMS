
      const params = new URLSearchParams(window.location.search);

      const email = params.get("email");
      const otp = params.get("otp");

      const emailInput = document.getElementById("email");
      const otpInput = document.getElementById("otp");
      const form = document.getElementById("resetPasswordForm");
      const message = document.getElementById("message");

      if (!email || !otp) {
        message.style.display = "block";
        message.className = "alert alert-danger";
        message.textContent =
          "Reset information is missing. Please request a new OTP.";

        form.style.display = "none";
      } else {
        emailInput.value = email;
        otpInput.value = otp;
      }

      form.addEventListener("submit", async function (event) {
        event.preventDefault();

        const newPassword = document.getElementById("newPassword").value;

        const confirmPassword =
          document.getElementById("confirmPassword").value;

        message.style.display = "none";

        if (newPassword !== confirmPassword) {
          message.style.display = "block";
          message.className = "alert alert-danger";
          message.textContent =
            "New password and confirm password do not match.";

          return;
        }

        if (newPassword.length < 8) {
          message.style.display = "block";
          message.className = "alert alert-danger";
          message.textContent = "Password must be at least 8 characters.";

          return;
        }

        try {
          const response = await fetch("/reset-password", {
            method: "POST",

            headers: {
              "Content-Type": "application/json",
            },

            body: JSON.stringify({
              email: email,
              otp: otp,
              new_password: newPassword,
              confirm_password: confirmPassword,
            }),
          });

          const data = await response.json();

          message.style.display = "block";

          if (response.ok && data.success) {
            message.className = "alert alert-success";
            message.textContent =
              data.message || "Password reset successfully.";

            form.reset();

            setTimeout(function () {
              window.location.href = "/login";
            }, 1500);
          } else {
            message.className = "alert alert-danger";
            message.textContent = data.message || "Unable to reset password.";
          }
        } catch (error) {
          message.style.display = "block";
          message.className = "alert alert-danger";
          message.textContent = "Unable to reset password. Please try again.";
        }
      });
    
