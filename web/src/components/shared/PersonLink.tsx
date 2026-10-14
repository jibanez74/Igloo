import { Link, useLocation } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { FOCUS_VISIBLE_RING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type PersonLinkProps = {
  personId: number;
  children: ReactNode;
  className?: string;
  label?: string;
};

export default function PersonLink({
  personId,
  children,
  className,
  label,
}: PersonLinkProps) {
  const returnTo = useLocation({ select: (location) => location.href });

  return (
    <Link
      to="/people/$id"
      params={{ id: String(personId) }}
      search={{ returnTo }}
      className={cn(
        "rounded-sm hover:text-primary",
        FOCUS_VISIBLE_RING_CLASS,
        className,
      )}
      aria-label={label}
    >
      {children}
    </Link>
  );
}
