/**
 * Standardized toast notification helpers for consistent messaging.
 *
 * These helpers provide a unified API for common toast patterns,
 * ensuring consistent wording and structure across the application.
 */

import { toast } from "sonner";
import { apiErrorMessage } from "@/lib/is-api-failure";
import type { ApiFailureType } from "@/types";

// ============================================================================
// Success Toasts
// ============================================================================

/**
 * Show a success toast for item creation.
 * @example showCreated("Playlist") → "Playlist created"
 */
export function showCreated(itemName: string, description?: string) {
  toast.success(`${itemName} created`, description ? { description } : undefined);
}

/**
 * Show a success toast for item updates.
 * @example showUpdated("Playlist") → "Playlist updated"
 */
export function showUpdated(itemName: string, description?: string) {
  toast.success(`${itemName} updated`, description ? { description } : undefined);
}

/**
 * Show a success toast for item deletion.
 * @example showDeleted("Playlist") → "Playlist deleted"
 */
export function showDeleted(itemName: string, description?: string) {
  toast.success(`${itemName} deleted`, description ? { description } : undefined);
}

/**
 * Show a success toast for adding items.
 * @example showAdded("Track", "to playlist") → "Track added to playlist"
 */
export function showAdded(itemName: string, target?: string, description?: string) {
  const message = target ? `${itemName} added ${target}` : `${itemName} added`;
  toast.success(message, description ? { description } : undefined);
}

/**
 * Show a success toast for removing items.
 * @example showRemoved("Track", "from playlist") → "Track removed from playlist"
 */
export function showRemoved(itemName: string, target?: string, description?: string) {
  const message = target ? `${itemName} removed ${target}` : `${itemName} removed`;
  toast.success(message, description ? { description } : undefined);
}

/**
 * Show a generic success toast.
 * @example showSuccess("Login successful")
 */
export function showSuccess(message: string, description?: string) {
  toast.success(message, description ? { description } : undefined);
}

// ============================================================================
// Error Toasts
// ============================================================================

/**
 * Show an error toast for failed actions. Pass the failed envelope rather than
 * its message: only a server-worded 4xx reaches the description, never a 5xx
 * constant or the client's canned 404/network text (`apiErrorMessage`).
 * @example showActionFailed("create playlist", res)
 * @example showActionFailed("play album", "Something went wrong. Please try again.")
 */
export function showActionFailed(action: string, failure?: string | ApiFailureType) {
  const description = typeof failure === "string" ? failure : apiErrorMessage(failure, "");
  toast.error(`Failed to ${action}`, description ? { description } : undefined);
}

/**
 * Show an error toast for validation errors.
 * @example showValidationError("Name is required")
 */
export function showValidationError(message: string) {
  toast.error("Validation error", { description: message });
}

/**
 * Show a generic error toast.
 * @example showError("Something went wrong")
 */
export function showError(message: string, description?: string) {
  toast.error(message, description ? { description } : undefined);
}

// ============================================================================
// Info Toasts
// ============================================================================

/**
 * Show an info toast for neutral information.
 * @example showInfo("Track already in playlist")
 */
export function showInfo(message: string, description?: string) {
  toast.info(message, description ? { description } : undefined);
}
