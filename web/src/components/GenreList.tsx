import type { Genre } from "@/types/Genre";

type GenreListProps = {
  genres: Genre[];
};

export default function GenreList({ genres }: GenreListProps) {
  return (
    <div className='flex flex-wrap gap-4 text-sm text-sky-200 mb-6 drop-shadow-lg bg-slate-900/20 backdrop-blur-sm rounded-lg px-4 py-2'>
      {genres.map(genre => (
        <span key={genre.id} className='px-2 py-0.5 bg-sky-500/10 rounded'>
          {genre.tag}
        </span>
      ))}
    </div>
  );
}
