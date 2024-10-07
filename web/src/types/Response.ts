export type Res<T> = {
  error: boolean;
  message: string;
  data?: T;
};
