(function () {
  function init() {
    const btn = document.getElementById("togglePassword");
    const input = document.getElementById("password");

    if (btn && input) {
      btn.addEventListener("click", e => {
        e.preventDefault();
        e.stopPropagation();

        const isPw = input.getAttribute("type") === "password";

        input.setAttribute("type", isPw ? "text" : "password");

        const icon = btn.querySelector("i");

        if (icon) {
          icon.classList.remove("fa-eye", "fa-eye-slash");

          if (isPw) {
            icon.classList.add("fa-eye-slash");
          } else {
            icon.classList.add("fa-eye");
          }
        }
      });
    }

    const card = document.getElementById("login-card");

    if (!card) return;
    const prefersReduced = window.matchMedia(
      "(prefers-reduced-motion: reduce)"
    ).matches;

    if (prefersReduced) {
      card.classList.remove("opacity-0", "translate-y-2");
      return;
    }

    requestAnimationFrame(() => {
      card.classList.remove("opacity-0", "translate-y-2");
    });

    const form = document.getElementById("login-form");
    if (!form) return;

    form.addEventListener("submit", async e => {
      e.preventDefault();

      const emailInput = document.getElementById("email");
      const passwordInput = document.getElementById("password");
      const submitButton = form.querySelector('button[type="submit"]');
      const submitIcon = submitButton?.querySelector("i");
      const submitText = submitButton?.querySelector("span");

      if (!emailInput || !passwordInput || !submitButton) return;

      const email = emailInput.value.trim();
      const password = passwordInput.value;

      const existingError = form.querySelector(".error-message");
      if (existingError) {
        existingError.remove();
      }

      // Disable form and show loading state
      emailInput.disabled = true;
      passwordInput.disabled = true;
      submitButton.disabled = true;

      if (submitIcon) {
        submitIcon.classList.remove("fa-right-to-bracket");
        submitIcon.classList.add("fa-spinner", "fa-spin");
      }

      if (submitText) {
        submitText.textContent = "Signing in...";
      }

      try {
        const response = await fetch("/api/v1/login", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            email: email,
            password: password,
          }),
        });

        const data = await response.json();

        if (!response.ok || data.error) {
          const errorMsg =
            data.message || "An error occurred. Please try again.";
          showError(form, errorMsg);
          return;
        }

        window.location.href = "/";
      } catch (error) {
        console.error("Login error:", error);
        showError(
          form,
          "Network error. Please check your connection and try again."
        );
      } finally {
        emailInput.disabled = false;
        passwordInput.disabled = false;
        submitButton.disabled = false;

        if (submitIcon) {
          submitIcon.classList.remove("fa-spinner", "fa-spin");
          submitIcon.classList.add("fa-right-to-bracket");
        }

        if (submitText) {
          submitText.textContent = "Sign in";
        }
      }
    });
  }

  function showError(form, message) {
    const existingError = form.querySelector(".error-message");

    if (existingError) {
      existingError.remove();
    }

    const errorDiv = document.createElement("div");
    errorDiv.className =
      "error-message rounded-lg bg-red-500/10 border border-red-500/20 p-3 text-sm text-red-400 flex items-center gap-2";
    errorDiv.setAttribute("role", "alert");
    errorDiv.innerHTML = `
      <i class="fa-solid fa-circle-exclamation" aria-hidden="true"></i>
      <span>${escapeHtml(message)}</span>
    `;

    const submitContainer = form.querySelector(".pt-2");

    if (submitContainer) {
      submitContainer.insertAdjacentElement("beforebegin", errorDiv);
    } else {
      form.appendChild(errorDiv);
    }

    const emailInput = document.getElementById("email");
    if (emailInput) {
      emailInput.focus();
    }
  }

  function escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }

  // Wait for DOM to be ready
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
