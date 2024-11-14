import { useWindowDimensions } from 'react-native';

export function useScale() {
  const { width: screenWidth } = useWindowDimensions();
  
  // Base scale on a reference screen width (e.g., 1920 for FHD)
  const baseWidth = 1920;
  const scale = screenWidth / baseWidth;
  
  return scale;
}
