import { FiFilm } from "react-icons/fi";
import type { Studio } from "@/types/Studio";

type StudioListProps = {
  studios: Studio[];
};

export default function StudioList({ studios }: StudioListProps) {
  if (studios.length === 0) return null;

  return (
    <div>
      <div className='flex items-center gap-1 text-sky-400 mb-1'>
        <FiFilm className='w-4 h-4' aria-hidden='true' />
        <span className='text-sm font-medium'>Studios</span>
      </div>
      <div className='text-white'>
        {studios.map((studio, index) => (
          <span key={studio.id}>
            {studio.name}
            {index < studios.length - 1 && ", "}
          </span>
        ))}
      </div>
    </div>
  );
} 