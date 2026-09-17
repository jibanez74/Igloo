// API RESPONSE TYPES
// Types for API responses and router context


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
  // HTTP status, set by the client for failures it synthesizes (404, network).
  status?: number;
  data?: never; // Explicitly no data on failure
};

// Union type representing all possible API responses
export type ApiResponseType<T extends Record<string, unknown>> =
  | ApiSuccessType<T>
  | ApiFailureType;
