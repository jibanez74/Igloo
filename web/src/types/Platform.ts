export type Browser = {
  name: "chrome" | "firefox" | "safari" | "edge" | "unknown";
  version: string;
  engine: "webkit" | "gecko" | "blink" | "unknown";
};
