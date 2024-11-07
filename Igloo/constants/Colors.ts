const primary = '#121F32';      // Deep blue-gray - main background
const secondary = '#1B2B45';    // Lighter blue-gray - focused items
const accent = '#243B61';       // Highlighted elements
const textPrimary = '#FFFFFF';  // White text
const textSecondary = '#B8C6D9'; // Light blue-gray text

export default {
  light: {
    text: textPrimary,
    background: primary,
    tint: textPrimary,
    tabIconDefault: textSecondary,
    tabIconSelected: textPrimary,
  },
  dark: {
    text: textPrimary,
    background: primary,
    tint: textPrimary,
    tabIconDefault: textSecondary,
    tabIconSelected: textPrimary,
  },
  // Main colors
  primary,
  secondary,
  accent,
  
  // Text colors
  textPrimary,
  textSecondary,
  
  // UI States
  focusedBackground: secondary,
  unfocusedBackground: primary,
  
  // Status colors
  error: '#FF4B4B',
  success: '#4CAF50',
  warning: '#FFC107',
  
  // Additional UI colors
  cardBackground: '#1A2737',
  borderColor: '#2A3C56',
  overlay: 'rgba(18, 31, 50, 0.9)',
};
