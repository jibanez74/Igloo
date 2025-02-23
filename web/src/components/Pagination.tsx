import { FiChevronLeft, FiChevronRight } from "react-icons/fi";

interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

export default function Pagination({
  currentPage,
  totalPages,
  onPageChange,
}: PaginationProps) {
  // Generate page numbers to display
  const getPageNumbers = () => {
    const pages = [];
    const showPages = 5; // Number of page buttons to show

    let startPage = Math.max(1, currentPage - Math.floor(showPages / 2));
    const endPage = Math.min(totalPages, startPage + showPages - 1);

    // Adjust start if we're near the end
    if (endPage - startPage + 1 < showPages) {
      startPage = Math.max(1, endPage - showPages + 1);
    }

    for (let i = startPage; i <= endPage; i++) {
      pages.push(i);
    }

    return pages;
  };

  return (
    <div className='flex items-center justify-center gap-2 mt-6'>
      {/* Previous button */}
      <button
        onClick={() => onPageChange(currentPage - 1)}
        disabled={currentPage === 1}
        className='p-2 text-sky-200 hover:text-white hover:bg-sky-500/10 rounded-lg disabled:opacity-50 disabled:hover:bg-transparent disabled:cursor-not-allowed transition-colors'
        aria-label='Previous page'
      >
        <FiChevronLeft className='w-5 h-5' />
      </button>

      {/* First page */}
      {getPageNumbers()[0] > 1 && (
        <>
          <button
            onClick={() => onPageChange(1)}
            className={`px-3 py-1 rounded-lg transition-colors ${
              currentPage === 1
                ? "bg-sky-500 text-white"
                : "text-sky-200 hover:text-white hover:bg-sky-500/10"
            }`}
          >
            1
          </button>
          {getPageNumbers()[0] > 2 && <span className='text-sky-200'>...</span>}
        </>
      )}

      {/* Page numbers */}
      {getPageNumbers().map(page => (
        <button
          key={page}
          onClick={() => onPageChange(page)}
          className={`px-3 py-1 rounded-lg transition-colors ${
            currentPage === page
              ? "bg-sky-500 text-white"
              : "text-sky-200 hover:text-white hover:bg-sky-500/10"
          }`}
        >
          {page}
        </button>
      ))}

      {/* Last page */}
      {getPageNumbers()[getPageNumbers().length - 1] < totalPages && (
        <>
          {getPageNumbers()[getPageNumbers().length - 1] < totalPages - 1 && (
            <span className='text-sky-200'>...</span>
          )}
          <button
            onClick={() => onPageChange(totalPages)}
            className={`px-3 py-1 rounded-lg transition-colors ${
              currentPage === totalPages
                ? "bg-sky-500 text-white"
                : "text-sky-200 hover:text-white hover:bg-sky-500/10"
            }`}
          >
            {totalPages}
          </button>
        </>
      )}

      {/* Next button */}
      <button
        onClick={() => onPageChange(currentPage + 1)}
        disabled={currentPage === totalPages}
        className='p-2 text-sky-200 hover:text-white hover:bg-sky-500/10 rounded-lg disabled:opacity-50 disabled:hover:bg-transparent disabled:cursor-not-allowed transition-colors'
        aria-label='Next page'
      >
        <FiChevronRight className='w-5 h-5' />
      </button>
    </div>
  );
}
