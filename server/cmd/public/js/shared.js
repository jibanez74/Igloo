(function () {
  "use strict";

  function normalizePath(path) {
    if (!path || typeof path !== "string") return "/";
    return "/" + path.replace(/^\/+|\/+$/g, "") || "/";
  }

  function isActiveRoute(route, currentPath) {
    const normalizedRoute = normalizePath(route);
    const normalizedCurrent = normalizePath(currentPath);

    if (normalizedRoute === "/") {
      return normalizedCurrent === "/";
    }

    return (
      normalizedCurrent === normalizedRoute ||
      normalizedCurrent.startsWith(normalizedRoute + "/")
    );
  }

  function updateLinkStyles(link, isActive) {
    if (!link) return;

    link.classList.toggle("text-white", isActive);
    link.classList.toggle("bg-slate-800/60", isActive);
    link.classList.toggle("font-semibold", isActive);
    link.classList.toggle("md:border-amber-400", isActive);

    if (isActive) {
      link.setAttribute("aria-current", "page");
    } else {
      link.removeAttribute("aria-current");
    }

    const icon = link.querySelector("i");

    if (icon) {
      icon.classList.toggle("text-amber-400", isActive);
      icon.classList.toggle("text-slate-400", !isActive);
    }
  }

  function initNavigation() {
    const currentPath = window.location.pathname;
    const navLinks = document.querySelectorAll("aside [data-route]");

    navLinks.forEach(link => {
      const route = link.getAttribute("data-route");

      if (route) {
        const isActive = isActiveRoute(route, currentPath);
        updateLinkStyles(link, isActive);
      }
    });
  }

  function waitForDOM() {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", initNavigation);
    } else {
      initNavigation();
    }
  }

  waitForDOM();
  window.addEventListener("popstate", initNavigation);
})();
