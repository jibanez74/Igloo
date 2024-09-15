import PropTypes from "prop-types";
import { NavLink } from "react-router-dom";

export default function Pagination({
  currentPage,
  totalPages,
  pageSize,
  totalItems,
  urlPrefix,
}) {
  currentPage = Number(currentPage);

  const pageNumbers = [];
  const maxPageButtons = 5;

  for (let i = 1; i <= totalPages; i++) {
    pageNumbers.push(i);
  }

  let startPage = Math.max(1, currentPage - Math.floor(maxPageButtons / 2));
  let endPage = Math.min(totalPages, startPage + maxPageButtons - 1);

  if (endPage - startPage + 1 < maxPageButtons) {
    startPage = Math.max(1, endPage - maxPageButtons + 1);
  }

  return (
    <nav
      className='flex items-center justify-between border-t border-gray-200 px-4 sm:px-0 py-3'
      aria-label='Pagination'
    >
      <div className='hidden sm:block'>
        <p className='text-sm text-gray-700'>
          Showing{" "}
          <span className='font-medium'>
            {Math.min((currentPage - 1) * pageSize + 1, totalItems)}
          </span>{" "}
          to{" "}
          <span className='font-medium'>
            {Math.min(currentPage * pageSize, totalItems)}
          </span>{" "}
          of <span className='font-medium'>{totalItems}</span> results
        </p>
      </div>
      <div className='flex flex-1 justify-between sm:justify-end'>
        <NavLink
          to={`${urlPrefix}/${currentPage > 1 ? currentPage - 1 : 1}`}
          className={({ isActive }) => `
            relative inline-flex items-center rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 ring-1 ring-inset ring-gray-300 hover:bg-gray-50
            ${currentPage === 1 ? "opacity-50 cursor-not-allowed" : ""}
            ${isActive ? "bg-indigo-600 text-white" : ""}
          `}
        >
          Previous
        </NavLink>
        <div className='hidden md:flex -space-x-px mx-2'>
          {startPage > 1 && (
            <>
              <NavLink
                to={`${urlPrefix}/1`}
                className={({ isActive }) => `
                relative inline-flex items-center px-4 py-2 text-sm font-semibold text-gray-900 ring-1 ring-inset ring-gray-300 hover:bg-gray-50
                ${isActive ? "bg-indigo-600 text-white" : ""}
              `}
              >
                1
              </NavLink>
              {startPage > 2 && (
                <span className='relative inline-flex items-center px-4 py-2 text-sm font-semibold text-gray-700'>
                  ...
                </span>
              )}
            </>
          )}
          {pageNumbers.slice(startPage - 1, endPage).map(number => (
            <NavLink
              key={number}
              to={`${urlPrefix}/${number}`}
              className={({ isActive }) => `
                relative inline-flex items-center px-4 py-2 text-sm font-semibold
                ${
                  isActive
                    ? "z-10 bg-indigo-600 text-white focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
                    : "text-gray-900 ring-1 ring-inset ring-gray-300 hover:bg-gray-50"
                }
              `}
            >
              {number}
            </NavLink>
          ))}
          {endPage < totalPages && (
            <>
              {endPage < totalPages - 1 && (
                <span className='relative inline-flex items-center px-4 py-2 text-sm font-semibold text-gray-700'>
                  ...
                </span>
              )}
              <NavLink
                to={`${urlPrefix}/${totalPages}`}
                className={({ isActive }) => `
                relative inline-flex items-center px-4 py-2 text-sm font-semibold text-gray-900 ring-1 ring-inset ring-gray-300 hover:bg-gray-50
                ${isActive ? "bg-indigo-600 text-white" : ""}
              `}
              >
                {totalPages}
              </NavLink>
            </>
          )}
        </div>
        <NavLink
          to={`${urlPrefix}/${
            currentPage < totalPages ? currentPage + 1 : totalPages
          }`}
          className={({ isActive }) => `
            relative inline-flex items-center rounded-md bg-white px-3 py-2 text-sm font-semibold text-gray-900 ring-1 ring-inset ring-gray-300 hover:bg-gray-50
            ${currentPage === totalPages ? "opacity-50 cursor-not-allowed" : ""}
            ${isActive ? "bg-indigo-600 text-white" : ""}
          `}
        >
          Next
        </NavLink>
      </div>
    </nav>
  );
}

Pagination.propTypes = {
  currentPage: PropTypes.oneOfType([PropTypes.number, PropTypes.string])
    .isRequired,
  totalPages: PropTypes.number.isRequired,
  pageSize: PropTypes.number.isRequired,
  totalItems: PropTypes.number.isRequired,
  urlPrefix: PropTypes.string.isRequired,
};
