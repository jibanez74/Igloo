import type { ReactNode } from "react";

type GallerySectionProps = {
  /** Stable key: the screenshot spec shoots `[data-gallery="<id>"]`. */
  id: string;
  title: string;
  children: ReactNode;
};

export function GallerySection({ id, title, children }: GallerySectionProps) {
  const headingId = `gallery-${id}`;

  return (
    <section
      data-gallery={id}
      aria-labelledby={headingId}
      className="rounded-xl border border-border bg-background p-6"
    >
      <h2
        id={headingId}
        className="mb-4 text-lg font-semibold tracking-tight text-foreground"
      >
        {title}
      </h2>
      <div className="flex flex-col gap-5">{children}</div>
    </section>
  );
}

type GalleryRowProps = {
  label: string;
  children: ReactNode;
};

export function GalleryRow({ label, children }: GalleryRowProps) {
  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <div className="flex flex-wrap items-center gap-3">{children}</div>
    </div>
  );
}
