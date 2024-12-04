import { Dimensions } from "react-native";

const { width: SCREEN_WIDTH, height: SCREEN_HEIGHT } = Dimensions.get("window");

const TV_BREAKPOINTS = {
  HD: 1280,
  FHD: 1920,
  UHD: 3840,
};

function getCardSize() {
  if (SCREEN_WIDTH >= TV_BREAKPOINTS.UHD) {
    return {
      width: 300,
      height: 450,
      spacing: 32,
      titleSize: 40,
      textSize: 24,
    };
  }

  if (SCREEN_WIDTH >= TV_BREAKPOINTS.FHD) {
    return {
      width: 240,
      height: 360,
      spacing: 24,
      titleSize: 32,
      textSize: 20,
    };
  }

  return {
    width: 200,
    height: 300,
    spacing: 20,
    titleSize: 24,
    textSize: 18,
  };
}

const CARD = getCardSize();

const SPACING = {
  xs: 4,
  sm: 8,
  md: 16,
  lg: 24,
  xl: 32,
  xxl: 48,
};

export const Layout = {
  window: {
    width: SCREEN_WIDTH,
    height: SCREEN_HEIGHT,
  },
  breakpoints: TV_BREAKPOINTS,
  card: CARD,
  spacing: SPACING,
  isHD: SCREEN_WIDTH >= TV_BREAKPOINTS.HD,
  isFHD: SCREEN_WIDTH >= TV_BREAKPOINTS.FHD,
  isUHD: SCREEN_WIDTH >= TV_BREAKPOINTS.UHD,
} as const;
