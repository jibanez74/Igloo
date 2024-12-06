import { View, Text, StyleSheet, Pressable } from "react-native";
import { danger, info, success, light, secondary } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";

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
    padding: Layout.spacing.lg,
    borderRadius: Layout.spacing.sm,
    borderWidth: 1,
    opacity: 0.9,
    alignItems: "center",
  },
  message: {
    fontSize: Layout.card.textSize,
    textAlign: "center",
    marginBottom: Layout.spacing.lg,
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
    paddingVertical: Layout.spacing.sm,
    paddingHorizontal: Layout.spacing.xl,
    borderRadius: Layout.spacing.xs,
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
    fontSize: Layout.card.textSize,
    fontWeight: "bold",
    textAlign: "center",
  },
});
