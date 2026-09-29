import {
  PLAYLIST_DESCRIPTION_MAX_LENGTH,
  PLAYLIST_NAME_MAX_LENGTH,
  USER_PASSWORD_MAX_BYTES,
  USER_PASSWORD_MIN_LENGTH,
} from "@/lib/constants";
import { codePointLength } from "@/lib/utils";

/**
 * The first length rule a new password breaks, worded around `label`
 * ("Password", "New password"), or null when it passes. Mirrors the server: a
 * minimum in code points and a maximum in UTF-8 bytes.
 */
export function passwordRuleError(
  password: string,
  label = "Password",
): string | null {
  if (codePointLength(password) < USER_PASSWORD_MIN_LENGTH) {
    return `${label} must be at least ${USER_PASSWORD_MIN_LENGTH} characters.`;
  }
  if (new TextEncoder().encode(password).length > USER_PASSWORD_MAX_BYTES) {
    return `${label} must be at most ${USER_PASSWORD_MAX_BYTES} UTF-8 bytes.`;
  }
  return null;
}

/**
 * The first rule a playlist's name or description breaks, or null when both
 * pass. Shared by the movie and music playlist forms.
 */
export function playlistFieldsError(
  name: string,
  description: string,
): string | null {
  const trimmedName = name.trim();
  if (!trimmedName) return "Playlist name is required";
  if (codePointLength(trimmedName) > PLAYLIST_NAME_MAX_LENGTH) {
    return `Playlist name is too long (max ${PLAYLIST_NAME_MAX_LENGTH} characters)`;
  }
  if (codePointLength(description.trim()) > PLAYLIST_DESCRIPTION_MAX_LENGTH) {
    return `Playlist description is too long (max ${PLAYLIST_DESCRIPTION_MAX_LENGTH} characters)`;
  }
  return null;
}
