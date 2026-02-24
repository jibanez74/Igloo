type MovieTaglineProps = {
  tagline?: string | null;
};

export default function MovieTagline({ tagline }: MovieTaglineProps) {
  if (!tagline) return null;
  return (
    <p className="mt-2 text-lg text-slate-400 italic">
      <q>{tagline}</q>
    </p>
  );
}
