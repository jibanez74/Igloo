import type { QueryClient } from "@tanstack/react-query";

export type RouterContextType = {
  queryClient: QueryClient;
};

export type SimpleAlbumType = {
  id: number;
  cover?: string;
  title: string;
};

export type SimpleMovieType = {
  id: number;
  title: string;
  thumb: string;
};
