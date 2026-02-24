import type { ReactNode } from "react";
import MovieBackdrop from "./MovieBackdrop";

type MovieDetailsLayoutProps = {
  backdropUrl?: string | null;
  children: ReactNode;
  className?: string;
};

export default function MovieDetailsLayout({
  backdropUrl,
  children,
  className = "",
}: MovieDetailsLayoutProps) {
  const hasBackdrop = !!backdropUrl;

  return (
    <article aria-labelledby="movie-title" className={className}>
      {hasBackdrop && <MovieBackdrop src={backdropUrl} />}
      <div className={hasBackdrop ? "relative z-10 -mt-32" : ""}>
        {children}
      </div>
    </article>
  );
}
