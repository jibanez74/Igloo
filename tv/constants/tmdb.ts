// TMDB Image Sizes (matching backend)
export const TMDB = {
  POSTER: {
    WIDTH: 500,    // w500 from backend
    RATIO: 1.5,    // 2:3 aspect ratio
  },
  BACKDROP: {
    WIDTH: 1280,   // w1280 from backend
    RATIO: 0.5625, // 16:9 aspect ratio
  },
} as const;

// Screen Display Sizes
export const SCREEN = {
  POSTER: {
    WIDTH: 200,
    get HEIGHT() { return this.WIDTH * TMDB.POSTER.RATIO },
  },
  BACKDROP: {
    WIDTH: 1280,
    get HEIGHT() { return this.WIDTH * TMDB.BACKDROP.RATIO },
  },
} as const; 