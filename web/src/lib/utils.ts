import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Counts Unicode code points, matching the server's rune-based length limits;
// String.length counts UTF-16 units and overcounts astral characters (emoji).
export function codePointLength(value: string) {
  return Array.from(value).length
}
