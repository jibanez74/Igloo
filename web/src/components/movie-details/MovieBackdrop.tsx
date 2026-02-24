type MovieBackdropProps = {
  src: string | null;
  alt?: string;
};

export default function MovieBackdrop({
  src,
  alt = "",
}: MovieBackdropProps) {
  return (
    <header
      className="relative -mx-4 sm:-mx-6 lg:-mx-8"
      aria-hidden="true"
    >
      {src ? (
        <img
          src={src}
          alt={alt}
          className="aspect-21/9 w-full object-cover object-top"
        />
      ) : (
        <div className="aspect-21/9 w-full bg-slate-800" />
      )}
      <div
        className="absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent"
        aria-hidden="true"
      />
    </header>
  );
}
