import { useState } from "react";
import { FiChevronDown, FiChevronUp } from "react-icons/fi";
import getImgSrc from "@/utils/getImgSrc";
import type { Crew } from "@/types/Crew";

type CrewSectionProps = {
  crew: Crew[];
};

export default function CrewSection({ crew }: CrewSectionProps) {
  const [showAllCrew, setShowAllCrew] = useState(false);
  const displayedCrew = showAllCrew ? crew : crew.slice(0, 6);

  return (
    <div>
      <h3 className='text-lg font-medium text-sky-200 mb-4'>Crew</h3>
      <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4'>
        {displayedCrew.map(crew => (
          <div key={crew.id} className='text-center'>
            <div className='aspect-[2/3] mb-2 rounded-lg bg-slate-800/50 overflow-hidden'>
              {crew.thumb ? (
                <img
                  src={getImgSrc(crew.thumb)}
                  alt={crew.name}
                  className='w-full h-full object-cover'
                  loading='lazy'
                />
              ) : (
                <div className='w-full h-full flex items-center justify-center bg-slate-800/50'>
                  <span className='text-sky-200/50'>No Image</span>
                </div>
              )}
            </div>
            <div className='text-sm font-medium text-white'>{crew.name}</div>
            <div className='text-xs text-sky-200'>{crew.job}</div>
          </div>
        ))}
      </div>
      {crew.length > 6 && (
        <div className='flex justify-center mt-6'>
          <button
            type='button'
            onClick={() => setShowAllCrew(!showAllCrew)}
            className='inline-flex items-center gap-2 px-4 py-2 bg-slate-800/50 hover:bg-slate-800 
                     text-sky-200 hover:text-sky-100 rounded-lg transition-colors text-sm font-medium'
          >
            {showAllCrew ? (
              <>
                Show Less <FiChevronUp className='w-4 h-4' aria-hidden='true' />
              </>
            ) : (
              <>
                Show All Crew ({crew.length}){" "}
                <FiChevronDown className='w-4 h-4' aria-hidden='true' />
              </>
            )}
          </button>
        </div>
      )}
    </div>
  );
}
