import {
  View,
  Text,
  StyleSheet,
  ActivityIndicator,
  Dimensions,
} from "react-native";
import { light, info } from "@/constants/Colors";

const { width: SCREEN_WIDTH } = Dimensions.get("window");

// Scale values based on screen width (1920 is FHD width)
const scale = SCREEN_WIDTH / 1920;
const SPACING = {
  lg: Math.round(24 * scale),
  xl: Math.round(32 * scale),
};

const FONT_SIZES = {
  large: Math.round(24 * scale),
};

type LoadingProps = {
  message?: string;
  showSpinner?: boolean;
};

export default function Loading({
  message = "Loading...",
  showSpinner = true,
}: LoadingProps) {
  return (
    <View style={styles.container}>
      {showSpinner && (
        <ActivityIndicator size='large' color={info} style={styles.spinner} />
      )}
      <Text style={styles.message}>{message}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    padding: SPACING.xl,
  },
  spinner: {
    marginBottom: SPACING.lg,
    transform: [{ scale: 1.5 }],
  },
  message: {
    fontSize: FONT_SIZES.large,
    color: light,
    textAlign: "center",
  },
});
