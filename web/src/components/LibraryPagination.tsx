import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";

type LibraryPaginationProps = {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
};

export default function LibraryPagination({
  currentPage,
  totalPages,
  onPageChange,
}: LibraryPaginationProps) {
  const handlePageChange = (page: number) => {
    if (page < 1 || page > totalPages || page === currentPage) {
      return;
    }

    onPageChange(page);
  };

  // Generate page numbers to display
  const getPageNumbers = () => {
    const pages: (number | "start-ellipsis" | "end-ellipsis")[] = [];
    const maxVisible = 5;

    if (totalPages <= maxVisible + 2) {
      // Show all pages if total is small
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      // Always show first page
      pages.push(1);

      if (currentPage > 3) {
        pages.push("start-ellipsis");
      }

      // Show pages around current
      const start = Math.max(2, currentPage - 1);
      const end = Math.min(totalPages - 1, currentPage + 1);

      for (let i = start; i <= end; i++) {
        pages.push(i);
      }

      if (currentPage < totalPages - 2) {
        pages.push("end-ellipsis");
      }

      // Always show last page
      pages.push(totalPages);
    }

    return pages;
  };

  const pageNumbers = getPageNumbers();

  return (
    <Pagination>
      <PaginationContent className="max-w-full flex-wrap justify-center gap-y-2">
        <PaginationItem>
          <PaginationPrevious
            onClick={() => handlePageChange(currentPage - 1)}
            disabled={currentPage === 1}
            className={currentPage === 1 ? undefined : "hover:bg-accent"}
          />
        </PaginationItem>

        {pageNumbers.map((page) =>
          page === "start-ellipsis" || page === "end-ellipsis" ? (
            <PaginationItem key={page}>
              <PaginationEllipsis />
            </PaginationItem>
          ) : (
            <PaginationItem key={page}>
              <PaginationLink
                onClick={() => handlePageChange(page)}
                isActive={page === currentPage}
                aria-label={
                  page === currentPage
                    ? `Page ${page}, current page`
                    : `Go to page ${page}`
                }
                className={
                  page === currentPage
                    ? "bg-primary text-primary-foreground hover:bg-primary/90"
                    : "cursor-pointer hover:bg-accent"
                }
              >
                {page}
              </PaginationLink>
            </PaginationItem>
          )
        )}

        <PaginationItem>
          <PaginationNext
            onClick={() => handlePageChange(currentPage + 1)}
            disabled={currentPage === totalPages}
            className={
              currentPage === totalPages ? undefined : "hover:bg-accent"
            }
          />
        </PaginationItem>
      </PaginationContent>
    </Pagination>
  );
}
