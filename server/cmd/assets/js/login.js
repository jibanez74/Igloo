(function () {
  // Toggle password visibility
  const btn = document.getElementById("togglePassword");
  const input = document.getElementById("password");
  if (btn && input) {
    btn.addEventListener("click", () => {
      const isPw = input.getAttribute("type") === "password";
      input.setAttribute("type", isPw ? "text" : "password");
      const icon = btn.querySelector("i");
      if (icon) {
        icon.classList.toggle("fa-eye", !isPw);
        icon.classList.toggle("fa-eye-slash", isPw);
      }
    });
  }

  // Fade-in card on load (respects reduced motion)
  const card = document.getElementById("login-card");
  if (!card) return;
  const prefersReduced = window.matchMedia(
    "(prefers-reduced-motion: reduce)"
  ).matches;
  if (prefersReduced) {
    card.classList.remove("opacity-0", "translate-y-2");
    return;
  }
  // Use rAF to ensure styles are applied before we remove classes
  requestAnimationFrame(() => {
    card.classList.remove("opacity-0", "translate-y-2");
  });
})();
