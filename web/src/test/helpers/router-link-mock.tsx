import type { ReactNode } from "react";
import { vi } from "vitest";

type LinkStubProps = {
  children: ReactNode;
  params?: Record<string, string | undefined>;
  search?: unknown;
  to?: string;
};

// A Link rendered without a router: the href carries the resolved path
// params and data-search the search the component passed, so a test can
// assert both without booting the route tree.
function AnchorLink({ children, params, search, to, ...props }: LinkStubProps) {
  const href =
    typeof to === "string"
      ? Object.entries(params ?? {}).reduce(
          (path, [key, value]) => path.replace(`$${key}`, value ?? ""),
          to,
        )
      : "#";

  return (
    <a href={href} data-search={JSON.stringify(search ?? null)} {...props}>
      {children}
    </a>
  );
}

/**
 * Module factory for component suites that render Links without a router:
 *
 *   vi.mock("@tanstack/react-router", async () =>
 *     (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
 *   );
 */
export async function routerWithAnchorLinks() {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return { ...actual, Link: AnchorLink };
}

/** The search a stubbed Link was rendered with. */
export function linkSearch(link: HTMLElement): unknown {
  return JSON.parse(link.getAttribute("data-search") ?? "null");
}
