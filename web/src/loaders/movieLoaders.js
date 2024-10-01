import { defer } from "react-router-dom";
import queryClient from "../utils/queryClient";

export async function getMoviesWithPagination({ params }) {
  let { page, pageSize, keyword } = params;
  const caching = ["movies"];

  if (!page) {
    page = "1";
    caching.push(page);
  }

  if (!pageSize) {
    pageSize = "24";
    caching.push(pageSize);
  }

  if (keyword) {
    caching.push(keyword);
  }

  return defer({
    data: await queryClient.fetchQuery({
      queryKey: caching,
      queryFn: async () => {
        try {
          const res = await fetch(
            `/api/v1/movie/all?pageSize=${pageSize}&page=${page}&keyword=${keyword}`
          );

          const r = await res.json();

          if (r.error) {
            throw new Error(`${res.status} - ${r.message}`);
          }

          return r.data;
        } catch (err) {
          console.error(err);
          throw new Error("unable to make request to get latest movies");
        }
      },
    }),
  });
}

export async function loader({ params }) {
  return defer({
    data: await queryClient.fetchQuery({
      queryKey: ["movie", params.id],
      queryFn: async () => {
        try {
          const res = await fetch(`/api/v1/movie/by-id/${params.id}`);

          const r = await res.json();

          if (r.error) {
            throw new Error(`${res.status} - ${r.message}`);
          }

          return r.data;
        } catch (err) {
          console.error(err);
          throw new Error("unable to make request to get latest movies");
        }
      },
    }),
  });
}
