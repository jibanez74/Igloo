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

// Joins a variable list of element ids into a single `aria-describedby` value,
// dropping falsy entries so callers can inline `cond && id`. Returns undefined
// when nothing is left, so the attribute is omitted rather than set to "".
export function describedBy(...ids: Array<string | false | null | undefined>) {
  const value = ids.filter(Boolean).join(" ")
  return value || undefined
}

// Two-letter avatar initials from a display name (first + last word), upper-cased.
// `fallback` is returned for a blank name (e.g. "U" for a user, "?" generic).
export function getInitials(name: string, fallback = "U"): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return fallback
  if (parts.length === 1) return parts[0][0].toUpperCase()
  return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase()
}
