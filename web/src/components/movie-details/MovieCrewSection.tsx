import type { MovieCrewMemberView } from "@/lib/movie-details-view";

type MovieCrewSectionProps = {
  director?: { name: string } | null;
  writers?: MovieCrewMemberView[];
};

export default function MovieCrewSection({
  director,
  writers = [],
}: MovieCrewSectionProps) {
  const hasDirector = !!director?.name;
  const hasWriters = writers.length > 0;
  if (!hasDirector && !hasWriters) return null;

  return (
    <section className="mt-6" aria-labelledby="crew-heading">
      <h2 id="crew-heading" className="sr-only">
        Key Crew
      </h2>
      <dl className="grid grid-cols-2 gap-4 sm:grid-cols-3">
        {hasDirector && (
          <div
            tabIndex={0}
            className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
            role="group"
            aria-label={`Director: ${director!.name}`}
          >
            <dt className="text-sm text-slate-400">Director</dt>
            <dd className="font-semibold text-white">{director!.name}</dd>
          </div>
        )}
        {writers.map(writer => (
          <div
            key={writer.id}
            tabIndex={0}
            className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
            role="group"
            aria-label={`${writer.job}: ${writer.name}`}
          >
            <dt className="text-sm text-slate-400">{writer.job}</dt>
            <dd className="font-semibold text-white">{writer.name}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
