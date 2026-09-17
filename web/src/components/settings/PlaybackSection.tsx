import type { ReactNode } from "react";

type PlaybackSectionProps = {
  title: string;
  titleId?: string;
  description: string;
  children: ReactNode;
};

/** One labelled row inside a playback settings card. */
export default function PlaybackSection({
  title,
  titleId,
  description,
  children,
}: PlaybackSectionProps) {
  return (
    <section className="py-5 first:pt-0 last:pb-0">
      <h3 id={titleId} className="text-sm font-semibold text-foreground">
        {title}
      </h3>
      <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      <div className="mt-3">{children}</div>
    </section>
  );
}
