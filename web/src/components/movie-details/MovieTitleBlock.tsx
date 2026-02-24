type MovieTitleBlockProps = {
  title: string;
  releaseYear?: number | null;
  releaseDate?: string | null;
};

export default function MovieTitleBlock({
  title,
  releaseYear,
  releaseDate,
}: MovieTitleBlockProps) {
  return (
    <h1
      id="movie-title"
      tabIndex={-1}
      className="text-3xl font-bold text-white outline-none md:text-4xl lg:text-5xl"
    >
      {title}
      {releaseYear != null && (
        <span className="ml-3 font-normal text-slate-400">
          (
          <time dateTime={releaseDate ?? undefined}>
            {releaseYear}
          </time>
          )
        </span>
      )}
    </h1>
  );
}
