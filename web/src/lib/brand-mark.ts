// The Igloo brand glyph, shared by the app icons (scripts/generate-icons.ts
// renders favicon.svg and the PNG/ICO rasters from it) and the in-app
// BrandMark component, so the tab icon and the sidebar can never diverge.
// Pure data only: this module is compiled under both tsconfig.app.json and
// tsconfig.node.json.

/** The icon canvas: the glyph plus the margins a tile needs around it. */
export const BRAND_MARK_VIEWBOX = "0 0 64 64";

/**
 * The glyph's own bounds (x 10–54, y 21–45), for inline use where the parent
 * supplies the tile and the mark should fill its box like the icon fills its
 * tile.
 */
export const BRAND_MARK_GLYPH_VIEWBOX = "10 21 44 24";

/**
 * An igloo on a 64-unit canvas: a semicircular dome (radius 22, centred at
 * (32, 43)) on a 2-unit ground course, spanning x 10–54 and y 21–45, with an
 * arched doorway notched out of its base. Ice-block seams — one horizontal
 * course line and staggered vertical joints, each 4 units so they hold a
 * pixel at 16 px — are cut through the dome as
 * `evenodd` holes that never overlap each other, so the same path reads on any
 * background. At 16 px the seams fade and only the hut silhouette remains.
 */
export const BRAND_MARK_PATH = [
  // Dome + ground course, with the doorway (x 26.5–37.5, apex y 35) on the base edge.
  "M10 45V43a22 22 0 0 1 44 0v2H37.5V40.5a5.5 5.5 0 0 0-11 0V45Z",
  // Horizontal course seam (y 31–35). Its ends run along the dome's own arc:
  // straight ends would leave a sliver of dome outside the cut.
  "M11.506 35H52.494A22 22 0 0 0 50.439 31H13.561A22 22 0 0 0 11.506 35Z",
  // Bottom course joints, stopping short of the ground.
  "M16.5 35V42.5H19.5V35Z",
  "M44.5 35V42.5H47.5V35Z",
  // Top course joints, staggered between the ones below.
  "M23.5 31V25H26.5V31Z",
  "M37.5 31V25H40.5V31Z",
].join("");
