import type { QueryClient } from "@tanstack/react-query";

export type ApiSuccessType<T extends Record<string, unknown>> = {
  error: false;
  message?: string;
  data: T; // always present on success
};

export type ApiFailureType = {
  error: true;
  message: string;
  data?: never; // explicitly *no* data
};

export type ApiResponseType<T extends Record<string, unknown>> =
  | ApiSuccessType<T>
  | ApiFailureType;

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
