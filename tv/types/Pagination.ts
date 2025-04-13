export type PaginationSearch = {
  page: number;
  limit: number;
};

export type PaginationResponse<T> = {
  count: number;
  current_page: number;
  total_pages: number;
  items: T[];
};
