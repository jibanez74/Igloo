import { For, Show } from "solid-js";
import { FiChevronLeft, FiChevronRight } from "solid-icons/fi";

type PaginationProps = {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
};

export default function Pagination(props: PaginationProps) {
  const { currentPage, totalPages, onPageChange } = props;

  const getPageNumbers = () => {
    const pages = [];
    const showPages = 10;

    let startPage = Math.max(1, currentPage - Math.floor(showPages / 2));
    const endPage = Math.min(totalPages, startPage + showPages - 1);

    if (endPage - startPage + 1 < showPages) {
      startPage = Math.max(1, endPage - showPages + 1);
    }

    for (let i = startPage; i <= endPage; i++) {
      pages.push(i);
    }

    return pages;
  };

  return (
    <div class="flex items-center justify-center gap-2 mt-6">
      <button
        onClick={() => onPageChange(currentPage - 1)}
        disabled={currentPage === 1}
        class="p-2 text-sky-200 hover:text-white hover:bg-sky-500/10 rounded-lg disabled:opacity-50 disabled:hover:bg-transparent disabled:cursor-not-allowed transition-colors"
        aria-label="Previous page"
      >
        <FiChevronLeft class="w-5 h-5" />
      </button>

      {/* First page */}
      <Show when={getPageNumbers()[0] > 1}>
        <button
          onClick={() => onPageChange(1)}
          class={`px-3 py-1 rounded-lg transition-colors ${
            currentPage === 1
              ? "bg-sky-500 text-white"
              : "text-sky-200 hover:text-white hover:bg-sky-500/10"
          }`}
        >
          1
        </button>
        <Show when={getPageNumbers()[0] > 2}>
          <span class="text-sky-200">...</span>
        </Show>
      </Show>

      {/* Page numbers */}
      <For each={getPageNumbers()}>
        {(page) => (
          <button
            onClick={() => onPageChange(page)}
            class={`px-3 py-1 rounded-lg transition-colors ${
              currentPage === page
                ? "bg-sky-500 text-white"
                : "text-sky-200 hover:text-white hover:bg-sky-500/10"
            }`}
          >
            {page}
          </button>
        )}
      </For>

      {/* Last page */}
      <Show when={getPageNumbers()[getPageNumbers().length - 1] < totalPages}>
        <Show when={getPageNumbers()[getPageNumbers().length - 1] < totalPages - 1}>
          <span class="text-sky-200">...</span>
        </Show>
        <button
          onClick={() => onPageChange(totalPages)}
          class={`px-3 py-1 rounded-lg transition-colors ${
            currentPage === totalPages
              ? "bg-sky-500 text-white"
              : "text-sky-200 hover:text-white hover:bg-sky-500/10"
          }`}
        >
          {totalPages}
        </button>
      </Show>

      {/* Next button */}
      <button
        onClick={() => onPageChange(currentPage + 1)}
        disabled={currentPage === totalPages}
        class="p-2 text-sky-200 hover:text-white hover:bg-sky-500/10 rounded-lg disabled:opacity-50 disabled:hover:bg-transparent disabled:cursor-not-allowed transition-colors"
        aria-label="Next page"
      >
        <FiChevronRight class="w-5 h-5" />
      </button>
    </div>
  );
}
