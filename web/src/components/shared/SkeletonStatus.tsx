import type { ReactNode } from "react";

type SkeletonStatusProps = {
  /** Spoken loading label, e.g. "Loading movies". */
  label: string;
  /** The placeholder geometry, hidden from assistive tech. */
  children: ReactNode;
};

/**
 * The loading-layout contract (design-system §3.4) for a skeleton that does
 * not carry its own: one `role="status"` named by `label`, with the muted
 * boxes under it hidden, so the wait is announced once instead of as a run of
 * empty elements.
 */
export default function SkeletonStatus({ label, children }: SkeletonStatusProps) {
  return (
    <div role="status" aria-label={label}>
      <span className="sr-only">{label}...</span>
      <div aria-hidden="true">{children}</div>
    </div>
  );
}
