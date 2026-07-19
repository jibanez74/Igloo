type MovieDetailsTitleHeadingProps = {
  title: string;
  releaseYear: number | null;
  releaseDateStr: string | null;
};

export default function MovieDetailsTitleHeading({
  title,
  releaseYear,
  releaseDateStr,
}: MovieDetailsTitleHeadingProps) {
  return (
    <h1
      id="movie-title"
      tabIndex={-1}
      className="flex w-full max-w-full min-w-0 flex-col gap-1 rounded-sm text-2xl font-bold wrap-break-word text-white drop-shadow-lg focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background sm:gap-0 sm:text-3xl lg:flex-row lg:flex-wrap lg:items-baseline lg:gap-x-3 lg:text-4xl xl:text-5xl"
    >
      <span className="min-w-0">{title}</span>
      {releaseYear != null && (
        <span className="shrink-0 font-normal text-white/80 sm:text-3xl lg:text-4xl xl:text-5xl">
          (
          <time dateTime={releaseDateStr ?? undefined}>{releaseYear}</time>)
        </span>
      )}
    </h1>
  );
}
