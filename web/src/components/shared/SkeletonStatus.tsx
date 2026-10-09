import type { ReactNode } from "react";

type SkeletonStatusProps = {
  /** Spoken loading label, e.g. "Loading movies". */
  label: string;
  /** Classes for the status box, typically the shared loading motion. */
  className?: string;
  /** The placeholder geometry, hidden from assistive tech. */
  children: ReactNode;
};

/**
 * The loading-layout contract (design-system §3.4) every skeleton renders
 * through: one `role="status"` named by `label`, with the muted boxes under it
 * hidden, so the wait is announced once instead of as a run of empty elements.
 */
export default function SkeletonStatus({
  label,
  className,
  children,
}: SkeletonStatusProps) {
  return (
    <div className={className} role="status" aria-label={label}>
      <span className="sr-only">{label}...</span>
      <div aria-hidden="true">{children}</div>
    </div>
  );
}
