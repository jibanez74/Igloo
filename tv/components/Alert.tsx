import { View, Text, StyleSheet, Pressable, Dimensions } from "react-native";
import { danger, info, success, light, secondary } from "@/constants/Colors";

const { width: SCREEN_WIDTH } = Dimensions.get("window");

// Scale values based on screen width (1920 is FHD width)
const scale = SCREEN_WIDTH / 1920;
const SPACING = {
  xs: Math.round(4 * scale),
  sm: Math.round(8 * scale),
  md: Math.round(16 * scale),
  lg: Math.round(24 * scale),
  xl: Math.round(32 * scale),
};

const FONT_SIZES = {
  small: Math.round(16 * scale),
  medium: Math.round(20 * scale),
  large: Math.round(24 * scale),
};

type AlertType = "error" | "info" | "success";

type AlertProps = {
  type?: AlertType;
  message: string;
  onDismiss?: () => void;
};

export default function Alert({
  type = "error",
  message,
  onDismiss,
}: AlertProps) {
  return (
    <View
      style={[
        styles.container,
        type === "error" && styles.errorContainer,
        type === "info" && styles.infoContainer,
        type === "success" && styles.successContainer,
      ]}
    >
      <Text
        style={[
          styles.message,
          type === "error" && styles.errorMessage,
          type === "info" && styles.infoMessage,
          type === "success" && styles.successMessage,
        ]}
      >
        {message}
      </Text>

      {onDismiss && (
        <Pressable
          focusable={true}
          style={({ focused }) => [
            styles.button,
            focused && styles.buttonFocused,
          ]}
          onPress={onDismiss}
        >
          <Text style={styles.buttonText}>OK</Text>
        </Pressable>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    padding: SPACING.lg,
    borderRadius: SPACING.sm,
    borderWidth: 1,
    opacity: 0.9,
    alignItems: "center",
  },
  message: {
    fontSize: FONT_SIZES.medium,
    textAlign: "center",
    marginBottom: SPACING.lg,
  },
  // Error styles
  errorContainer: {
    backgroundColor: `${danger}33`,
    borderColor: danger,
  },
  errorMessage: {
    color: light,
  },
  // Info styles
  infoContainer: {
    backgroundColor: `${info}33`,
    borderColor: info,
  },
  infoMessage: {
    color: light,
  },
  // Success styles
  successContainer: {
    backgroundColor: `${success}33`,
    borderColor: success,
  },
  successMessage: {
    color: light,
  },
  // Button styles
  button: {
    paddingVertical: SPACING.sm,
    paddingHorizontal: SPACING.xl,
    borderRadius: SPACING.xs,
    backgroundColor: `${light}33`,
    borderWidth: 2,
    borderColor: "transparent",
  },
  buttonFocused: {
    backgroundColor: `${light}66`,
    borderColor: secondary,
    transform: [{ scale: 1.1 }],
  },
  buttonText: {
    color: light,
    fontSize: FONT_SIZES.medium,
    fontWeight: "bold",
    textAlign: "center",
  },
});
