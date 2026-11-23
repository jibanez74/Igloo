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
 * Module factory for suites that render without a router: the real module
 * with `overrides` swapped in, such as a `useNavigate` the test spies on.
 *
 *   vi.mock("@tanstack/react-router", async () =>
 *     (await import("../helpers/router-link-mock")).routerWith({
 *       useNavigate: () => mocks.navigate,
 *     }),
 *   );
 */
export async function routerWith(
  overrides: Partial<typeof import("@tanstack/react-router")>,
) {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return { ...actual, ...overrides };
}

/**
 * Module factory for component suites that render Links without a router:
 *
 *   vi.mock("@tanstack/react-router", async () =>
 *     (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
 *   );
 */
export async function routerWithAnchorLinks(
  overrides: Partial<typeof import("@tanstack/react-router")> = {},
) {
  // The stub takes the props the suites pass, not Link's full generic API.
  const Link = AnchorLink as unknown as typeof import("@tanstack/react-router").Link;
  return routerWith({ Link, ...overrides });
}

/** The search a stubbed Link was rendered with. */
export function linkSearch(link: HTMLElement): unknown {
  return JSON.parse(link.getAttribute("data-search") ?? "null");
}
