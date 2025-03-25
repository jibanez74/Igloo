import { For, Show } from "solid-js";
import { FiChevronLeft, FiChevronRight } from "solid-icons/fi";

type PaginationProps = {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
};

export default function Pagination(props: PaginationProps) {
  const getPageNumbers = () => {
    const pages = [];
    const showPages = 10;

    let startPage = Math.max(1, props.currentPage - Math.floor(showPages / 2));
    const endPage = Math.min(props.totalPages, startPage + showPages - 1);

    if (endPage - startPage + 1 < showPages) {
      startPage = Math.max(1, endPage - showPages + 1);
    }

    for (let i = startPage; i <= endPage; i++) {
      pages.push(i);
    }

    return pages;
  };

  return (
    <nav aria-label="Pagination" class="mt-6">
      <div class="flex items-center justify-center gap-2">
        <button
          onClick={() => props.onPageChange(props.currentPage - 1)}
          disabled={props.currentPage === 1}
          class="p-2 text-white hover:text-yellow-300 hover:bg-blue-800/50 rounded-lg 
                 disabled:opacity-50 disabled:hover:bg-transparent disabled:cursor-not-allowed 
                 disabled:text-blue-200 transition-colors"
          aria-label="Previous page"
        >
          <FiChevronLeft class="w-5 h-5" />
        </button>

        {/* First page */}
        <Show when={getPageNumbers()[0] > 1}>
          <button
            onClick={() => props.onPageChange(1)}
            class={`px-3 py-1 rounded-lg transition-colors ${
              props.currentPage === 1
                ? "bg-yellow-300 text-blue-950 font-semibold ring-2 ring-yellow-300/50"
                : "text-white hover:text-yellow-300 hover:bg-blue-800/50"
            }`}
            aria-label="Page 1"
            aria-current={props.currentPage === 1 ? "page" : undefined}
          >
            1
          </button>
          <Show when={getPageNumbers()[0] > 2}>
            <span class="text-white/50" aria-hidden="true">
              ...
            </span>
          </Show>
        </Show>

        {/* Page numbers */}
        <For each={getPageNumbers()}>
          {(page) => (
            <button
              onClick={() => props.onPageChange(page)}
              class={`px-3 py-1 rounded-lg transition-colors ${
                props.currentPage === page
                  ? "bg-yellow-300 text-blue-950 font-semibold ring-2 ring-yellow-300/50"
                  : "text-white hover:text-yellow-300 hover:bg-blue-800/50"
              }`}
              aria-label={`Page ${page}`}
              aria-current={props.currentPage === page ? "page" : undefined}
            >
              {page}
            </button>
          )}
        </For>

        {/* Last page */}
        <Show when={getPageNumbers()[getPageNumbers().length - 1] < props.totalPages}>
          <Show
            when={
              getPageNumbers()[getPageNumbers().length - 1] < props.totalPages - 1
            }
          >
            <span class="text-white/50" aria-hidden="true">
              ...
            </span>
          </Show>
          <button
            onClick={() => props.onPageChange(props.totalPages)}
            class={`px-3 py-1 rounded-lg transition-colors ${
              props.currentPage === props.totalPages
                ? "bg-yellow-300 text-blue-950 font-semibold ring-2 ring-yellow-300/50"
                : "text-white hover:text-yellow-300 hover:bg-blue-800/50"
            }`}
            aria-label={`Page ${props.totalPages}`}
            aria-current={props.currentPage === props.totalPages ? "page" : undefined}
          >
            {props.totalPages}
          </button>
        </Show>

        {/* Next button */}
        <button
          onClick={() => props.onPageChange(props.currentPage + 1)}
          disabled={props.currentPage === props.totalPages}
          class="p-2 text-white hover:text-yellow-300 hover:bg-blue-800/50 rounded-lg 
                 disabled:opacity-50 disabled:hover:bg-transparent disabled:cursor-not-allowed 
                 disabled:text-blue-200 transition-colors"
          aria-label="Next page"
        >
          <FiChevronRight class="w-5 h-5" />
        </button>
      </div>
    </nav>
  );
}
