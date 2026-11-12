import type { ComponentProps } from "react";
import {
  PLAYER_ICON_BUTTON_CLASS,
  PLAYER_PRIMARY_BUTTON_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

// The transport buttons of a video surface, shared by the player chrome and
// the watch room so both read as one player (docs/design-system.md §3.5).
// They stay plain buttons on the PLAYER_* recipes rather than Button
// variants: the player's controls are tuned together (sizes, the muted rest,
// the inert state) and the native sliders beside them are not Buttons either.

/** Seek, fullscreen: a 40 px muted circle that fills on hover. */
export function PlayerIconButton({
  className,
  type = "button",
  ...props
}: ComponentProps<"button">) {
  return (
    <button
      type={type}
      className={cn(PLAYER_ICON_BUTTON_CLASS, "size-10 hover:bg-accent", className)}
      {...props}
    />
  );
}

/** Play and pause: the 56 px primary circle with the glow. */
export function PlayerPlayButton({
  className,
  type = "button",
  ...props
}: ComponentProps<"button">) {
  return (
    <button
      type={type}
      className={cn(PLAYER_PRIMARY_BUTTON_CLASS, "size-14 shadow-glow", className)}
      {...props}
    />
  );
}
