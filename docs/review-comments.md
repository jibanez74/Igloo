# Review Comments

The patch introduces a light-theme contrast regression for actionless movie heroes and can display inaccurate audio capability badges. Both are user-visible defects in the newly added behavior.

## P1: Keep actionless hero text above the light fade

**Location:** `web/src/lib/constants.ts:413-414`

When an in-theaters movie has no YouTube trailer, the metadata or genres become the bottom-most hero content. They sit only `pb-6` above this opaque `from-background` fade, so in the light theme the hardcoded white text is rendered over a near-white surface and loses readable contrast.

Reserve space for the fade when `actionsSlot` is absent, or keep a dark scrim behind all hero text.

## P2: Stop guessing 5.1/7.1 for unknown surround layouts

**Location:** `web/src/lib/media-capabilities.ts:51-57`

When ffprobe reports a 6.1 seven-channel track or another unrecognized surround layout, `describePlaybackChannelLayout` returns `Surround`, after which this fallback labels every 6–7-channel stream as 5.1 and every 8+-channel stream as 7.1. The badge therefore reports a channel layout the source may not have.

Preserve recognized layouts, or use a generic surround label instead of guessing.
