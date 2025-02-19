import { useState } from "react";
import { FiChevronDown, FiChevronUp } from "react-icons/fi";
import getImgSrc from "@/utils/getImgSrc";
import type { Cast } from "@/types/Cast";

type CastSectionProps = {
  cast: Cast[];
};

export default function CastSection({ cast }: CastSectionProps) {
  const [showAllCast, setShowAllCast] = useState(false);
  const displayedCast = showAllCast ? cast : cast.slice(0, 6);

  return (
    <div className='mb-12'>
      <h3 className='text-lg font-medium text-sky-200 mb-4'>Cast</h3>
      <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4'>
        {displayedCast.map(cast => (
          <div key={cast.id} className='text-center'>
            <div className='aspect-[2/3] mb-2 rounded-lg bg-slate-800/50 overflow-hidden'>
              {cast.thumb ? (
                <img
                  src={getImgSrc(cast.thumb)}
                  alt={cast.name}
                  className='w-full h-full object-cover'
                  loading='lazy'
                />
              ) : (
                <div className='w-full h-full flex items-center justify-center bg-slate-800/50'>
                  <span className='text-sky-200/50'>No Image</span>
                </div>
              )}
            </div>
            <div className='text-sm font-medium text-white'>{cast.name}</div>
            <div className='text-xs text-sky-200'>{cast.character}</div>
          </div>
        ))}
      </div>
      {cast.length > 6 && (
        <div className='flex justify-center mt-6'>
          <button
            type='button'
            onClick={() => setShowAllCast(!showAllCast)}
            className='inline-flex items-center gap-2 px-4 py-2 bg-slate-800/50 hover:bg-slate-800 
                     text-sky-200 hover:text-sky-100 rounded-lg transition-colors text-sm font-medium'
          >
            {showAllCast ? (
              <>
                Show Less <FiChevronUp className='w-4 h-4' aria-hidden='true' />
              </>
            ) : (
              <>
                Show All Cast ({cast.length}){" "}
                <FiChevronDown className='w-4 h-4' aria-hidden='true' />
              </>
            )}
          </button>
        </div>
      )}
    </div>
  );
}
