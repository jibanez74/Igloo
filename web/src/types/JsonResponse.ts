export type JsonResponse<T> = {
  error: boolean;
  message: string;
  data: T;
};
