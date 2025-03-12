import { For } from "solid-js";
import getImgSrc from "../utils/getImgSrc";
import type { Cast } from "../types/Cast";

type CastSectionProps = {
  cast: Cast[];
};

export default function CastSection(props: CastSectionProps) {
  return (
    <div class="mb-12">
      <h3 class="text-lg font-medium text-sky-200 mb-4">Cast</h3>

      <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
        <For each={props.cast}>
          {(castMember) => (
            <div class="text-center">
              <div class="aspect-[2/3] mb-2 rounded-lg bg-slate-800/50 overflow-hidden">
                {castMember.thumb ? (
                  <img
                    src={getImgSrc(castMember.thumb)}
                    alt={castMember.name}
                    class="w-full h-full object-cover"
                    loading="lazy"
                  />
                ) : (
                  <div class="w-full h-full flex items-center justify-center bg-slate-800/50">
                    <span class="text-sky-200/50">No Image</span>
                  </div>
                )}
              </div>
              <div class="text-sm font-medium text-white">
                {castMember.name}
              </div>
              <div class="text-xs text-sky-200">{castMember.character}</div>
            </div>
          )}
        </For>
      </div>
    </div>
  );
}
