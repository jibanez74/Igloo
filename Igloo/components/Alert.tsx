import { useState, useEffect } from "react";
import {
  StyleSheet,
  View,
  Text,
  Animated,
  Dimensions,
  Pressable,
} from "react-native";
import Colors from "@/constants/Colors";

export type AlertProps = {
  title?: string;
  msg?: string;
  variant?: "danger" | "success" | "warning";
  time?: number;
  onDismiss?: () => void;
};

export default function Alert({
  title = "Error",
  msg = "An error occurred",
  time = 5000,
  variant = "danger",
  onDismiss,
}: AlertProps) {
  const [show, setShow] = useState(true);
  const fadeAnim = useState(new Animated.Value(0))[0];

  useEffect(() => {
    // Fade in
    Animated.timing(fadeAnim, {
      toValue: 1,
      duration: 300,
      useNativeDriver: true,
    }).start();

    // Set timeout to fade out
    const timer = setTimeout(() => {
      Animated.timing(fadeAnim, {
        toValue: 0,
        duration: 300,
        useNativeDriver: true,
      }).start(() => {
        setShow(false);
        onDismiss?.();
      });
    }, time - 300); // Subtract animation duration

    return () => clearTimeout(timer);
  }, [time, fadeAnim, onDismiss]);

  if (!show) return null;

  const getVariantStyles = () => {
    switch (variant) {
      case "success":
        return styles.success;
      case "warning":
        return styles.warning;
      case "danger":
      default:
        return styles.danger;
    }
  };

  const getVariantTextColor = () => {
    switch (variant) {
      case "success":
        return Colors.success;
      case "warning":
        return Colors.warning;
      case "danger":
      default:
        return Colors.error;
    }
  };

  return (
    <View style={styles.overlay}>
      <Animated.View
        style={[styles.container, getVariantStyles(), { opacity: fadeAnim }]}
      >
        <Pressable
          style={styles.content}
          focusable
          onPress={() => {
            setShow(false);
            onDismiss?.();
          }}
        >
          <Text style={[styles.title, { color: getVariantTextColor() }]}>
            {title}
          </Text>
          <Text style={styles.message}>{msg}</Text>
        </Pressable>
      </Animated.View>
    </View>
  );
}

const { width: SCREEN_WIDTH } = Dimensions.get("window");

const styles = StyleSheet.create({
  overlay: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
    zIndex: 1000,
    backgroundColor: "rgba(0,0,0,0.5)",
  },
  container: {
    width: SCREEN_WIDTH * 0.5, // 50% of screen width
    padding: 24,
    borderRadius: 12,
    shadowColor: "#000",
    shadowOffset: {
      width: 0,
      height: 2,
    },
    shadowOpacity: 0.25,
    shadowRadius: 3.84,
    elevation: 5,
  },
  content: {
    padding: 8,
  },
  title: {
    fontSize: 28,
    fontWeight: "bold",
    marginBottom: 12,
  },
  message: {
    fontSize: 20,
    color: Colors.textPrimary,
  },
  danger: {
    backgroundColor: Colors.primary,
    borderLeftWidth: 4,
    borderLeftColor: Colors.error,
  },
  success: {
    backgroundColor: Colors.primary,
    borderLeftWidth: 4,
    borderLeftColor: Colors.success,
  },
  warning: {
    backgroundColor: Colors.primary,
    borderLeftWidth: 4,
    borderLeftColor: Colors.warning,
  },
});
