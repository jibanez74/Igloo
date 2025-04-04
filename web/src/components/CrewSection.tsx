import { For, createMemo } from "solid-js";
import getImgSrc from "../utils/getImgSrc";
import sortCrew from "../utils/sortCrew";
import type { Crew } from "../types/Crew";

type CrewSectionProps = {
  crew: Crew[];
};

export default function CrewSection(props: CrewSectionProps) {
  const sortedCrew = createMemo(() => sortCrew(props.crew, true));

  return (
    <div class="mb-12">
      <h3 class="text-lg font-medium text-yellow-300 mb-4">Crew</h3>
      <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
        <For each={sortedCrew()}>
          {(crewMember) => (
            <div class="text-center">
              <div class="aspect-[2/3] mb-2 rounded-lg bg-blue-950/50 backdrop-blur-sm overflow-hidden shadow-lg shadow-blue-900/20">
                {crewMember.thumb ? (
                  <img
                    src={getImgSrc(crewMember.thumb)}
                    alt={crewMember.name}
                    class="w-full h-full object-cover"
                    loading="lazy"
                  />
                ) : (
                  <div class="w-full h-full flex items-center justify-center bg-blue-950/50">
                    <span class="text-blue-200/50">No Image</span>
                  </div>
                )}
              </div>
              <div class="text-sm font-medium text-white">
                {crewMember.name}
              </div>
              <div class="text-xs text-blue-200">{crewMember.job}</div>
            </div>
          )}
        </For>
      </div>
    </div>
  );
}
