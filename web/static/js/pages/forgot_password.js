
      document
        .getElementById("forgotPasswordForm")
        .addEventListener("submit", async function (event) {
          event.preventDefault();

          const email = document.getElementById("email").value.trim();

          const message = document.getElementById("message");

          message.style.display = "none";

          try {
            const response = await fetch("/forgot-password", {
              method: "POST",

              headers: {
                "Content-Type": "application/json",
              },

              body: JSON.stringify({
                email: email,
              }),
            });

            const data = await response.json();

            message.style.display = "block";
            message.textContent = data.message || "Request completed.";

            if (response.ok && data.success) {
              window.location.href =
                "/verify-reset-otp?email=" + encodeURIComponent(email);
            }
          } catch (error) {
            message.style.display = "block";

            message.textContent =
              "Unable to process the request. Please try again.";
          }
        });
    
