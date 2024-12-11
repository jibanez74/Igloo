import axios, { AxiosError } from "axios";

export default function getError(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError<{ error?: string }>;

    // Handle network errors
    if (!axiosError.response) {
      return "Unable to reach the server. Please check your internet connection.";
    }

    // Get the error message from the response if available
    const serverError = axiosError.response.data?.error;
    if (serverError) {
      return serverError;
    }

    // Handle common HTTP status codes
    switch (axiosError.response.status) {
      case 400:
        return "Invalid request. Please check your input.";
      case 401:
        return "You need to login to access this content.";
      case 403:
        return "You don't have permission to access this content.";
      case 404:
        return "The requested content was not found.";
      case 429:
        return "Too many requests. Please try again later.";
      case 500:
        return "Internal server error. Please try again later.";
      default:
        return "An unexpected error occurred. Please try again.";
    }
  }

  // Handle other types of errors
  if (error instanceof Error) {
    return error.message;
  }

  // Fallback error message
  return "An unexpected error occurred. Please try again.";
}
