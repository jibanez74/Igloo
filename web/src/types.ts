import type { QueryClient } from "@tanstack/react-query";

export type RouterContextType = {
  queryClient: QueryClient;
};

export type SimpleMovieType = {
  id: number;
  title: string;
  thumb: string;
};
