// API RESPONSE TYPES
// Types for API responses and router context

import type { QueryClient } from "@tanstack/react-query";
import type { AudioPlayerContextType } from "./audio-player";

// Successful API response structure
export type ApiSuccessType<T extends Record<string, unknown>> = {
  error: false;
  message?: string;
  data: T;
};

// Failed API response structure
export type ApiFailureType = {
  error: true;
  message: string;
  data?: never; // Explicitly no data on failure
};

// Union type representing all possible API responses
export type ApiResponseType<T extends Record<string, unknown>> =
  | ApiSuccessType<T>
  | ApiFailureType;

// ROUTER CONTEXT TYPES

// Context provided to TanStack Router
export type RouterContextType = {
  queryClient: QueryClient;
  audioPlayer: AudioPlayerContextType;
};
