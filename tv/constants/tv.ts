import { Dimensions } from "react-native";
import { TMDB } from "./tmdb";

const { width: WINDOW_WIDTH, height: WINDOW_HEIGHT } = Dimensions.get("window");

// Common TV breakpoints
const TV_BREAKPOINTS = {
  HD: 1280, // 720p
  FHD: 1920, // 1080p
  UHD: 3840, // 4K
} as const;

// Calculate poster sizes based on screen width
const calculatePosterSize = () => {
  if (WINDOW_WIDTH >= TV_BREAKPOINTS.UHD) {
    return {
      width: 300,
      columns: 6,
    };
  }
  if (WINDOW_WIDTH >= TV_BREAKPOINTS.FHD) {
    return {
      width: 240,
      columns: 6,
    };
  }
  // Default HD
  return {
    width: 200,
    columns: 5,
  };
};

const TV_LAYOUT = {
  SCREEN: {
    WIDTH: WINDOW_WIDTH,
    HEIGHT: WINDOW_HEIGHT,
  },
  GRID: {
    PADDING: 32,
    SPACING: 16,
    ...calculatePosterSize(),
  },
  POSTER: {
    get WIDTH() {
      return calculatePosterSize().width;
    },
    get HEIGHT() {
      return this.WIDTH * TMDB.POSTER.RATIO;
    },
  },
} as const;

export { TV_LAYOUT, TV_BREAKPOINTS };
