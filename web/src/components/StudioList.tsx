import { For } from "solid-js";
import { FiFilm } from "solid-icons/fi";
import type { Studio } from "../types/Studio";

type StudioListProps = {
  studios: Studio[];
};

export default function StudioList(props: StudioListProps) {
  if (props.studios.length === 0) return null;

  return (
    <div>
      <div class="flex items-center gap-1 text-sky-400 mb-1">
        <FiFilm class="w-4 h-4" aria-hidden={true} />
        <span class="text-sm font-medium">Studios</span>
      </div>

      <div class="text-white">
        <For each={props.studios}>
          {(studio, index) => (
            <span>
              {studio.name}
              {index() < props.studios.length - 1 && ", "}
            </span>
          )}
        </For>
      </div>
    </div>
  );
}
