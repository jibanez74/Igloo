import { Link } from "@tanstack/react-router";
import { FiChevronLeft, FiChevronRight } from "react-icons/fi";

type PaginationProps = {
  currentPage: number;
  totalPages: number;
  baseUrl: string;
  searchParams?: Record<string, string>;
};

export default function Pagination({
  currentPage,
  totalPages,
  baseUrl,
  searchParams = {},
}: PaginationProps) {
  const getPageUrl = (page: number) => {
    const params = new URLSearchParams(searchParams);
    params.set("page", page.toString());
    return `${baseUrl}?${params.toString()}`;
  };

  const renderPageNumbers = () => {
    const pages: (number | string)[] = [];
    const maxVisiblePages = 5;

    if (totalPages <= maxVisiblePages) {
      // Show all pages if total is less than or equal to max visible
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      // Always show first page
      pages.push(1);

      // Calculate start and end of visible pages
      let start = Math.max(2, currentPage - 1);
      let end = Math.min(totalPages - 1, currentPage + 1);

      // Adjust if we're near the start
      if (currentPage <= 3) {
        end = 4;
      }

      // Adjust if we're near the end
      if (currentPage >= totalPages - 2) {
        start = totalPages - 3;
      }

      // Add ellipsis if needed at the start
      if (start > 2) {
        pages.push("...");
      }

      // Add middle pages
      for (let i = start; i <= end; i++) {
        pages.push(i);
      }

      // Add ellipsis if needed at the end
      if (end < totalPages - 1) {
        pages.push("...");
      }

      // Always show last page
      pages.push(totalPages);
    }

    return pages.map((page, index) => {
      if (page === "...") {
        return (
          <span
            key={`ellipsis-${index}`}
            className='px-3 py-2 text-sky-200'
            aria-hidden='true'
          >
            ...
          </span>
        );
      }

      const isCurrentPage = page === currentPage;
      return (
        <Link
          key={page}
          to={baseUrl}
          search={{ ...searchParams, page: page.toString() }}
          className={`px-3 py-2 text-sm font-medium rounded-md transition-colors ${
            isCurrentPage
              ? "bg-sky-500/20 text-white"
              : "text-sky-200 hover:bg-sky-500/10"
          }`}
          aria-current={isCurrentPage ? "page" : undefined}
        >
          {page}
        </Link>
      );
    });
  };

  if (totalPages <= 1) return null;

  return (
    <nav
      className='flex items-center justify-center gap-1'
      aria-label='Pagination'
    >
      <Link
        to={baseUrl}
        search={{ ...searchParams, page: (currentPage - 1).toString() }}
        className={`p-2 text-sm font-medium rounded-md transition-colors ${
          currentPage === 1
            ? "text-sky-200/50 cursor-not-allowed"
            : "text-sky-200 hover:bg-sky-500/10"
        }`}
        aria-label='Previous page'
        disabled={currentPage === 1}
      >
        <FiChevronLeft className='w-5 h-5' aria-hidden='true' />
      </Link>

      <div className='flex items-center gap-1'>{renderPageNumbers()}</div>

      <Link
        to={baseUrl}
        search={{ ...searchParams, page: (currentPage + 1).toString() }}
        className={`p-2 text-sm font-medium rounded-md transition-colors ${
          currentPage === totalPages
            ? "text-sky-200/50 cursor-not-allowed"
            : "text-sky-200 hover:bg-sky-500/10"
        }`}
        aria-label='Next page'
        disabled={currentPage === totalPages}
      >
        <FiChevronRight className='w-5 h-5' aria-hidden='true' />
      </Link>
    </nav>
  );
}
