import { AxiosError } from "axios";

export function getErrorMessage(error: unknown): string {
  if (error instanceof AxiosError) {
    if (error.response) {
      const message = error.response.data?.error || error.message;
      return `${error.response.status} - ${message}`;
    } else if (error.request) {
      return "No response received from server";
    } else {
      return error.message || "Error setting up the request";
    }
  }

  if (error instanceof Error) {
    return error.message;
  }

  return String(error) || "An unknown error occurred";
}

export async function handleApiError<T>(
  fn: () => Promise<T>,
  errorHandler?: (error: unknown) => void
): Promise<T> {
  try {
    return await fn();
  } catch (error) {
    const errorMessage = getErrorMessage(error);

    if (errorHandler) {
      errorHandler(error);
    } else {
      console.error("API Error:", errorMessage);
    }

    throw new Error(errorMessage);
  }
}
