import { StyleSheet, Modal, Pressable } from "react-native";
import { Colors } from "@/constants/Colors";
import { ThemedView } from "./ThemedView";
import { ThemedText } from "./ThemedText";

interface AlertProps {
  visible: boolean;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function Alert({
  visible,
  title,
  message,
  confirmText = "Confirm",
  cancelText = "Cancel",
  onConfirm,
  onCancel,
}: AlertProps) {
  return (
    <Modal
      visible={visible}
      transparent={true}
      animationType="fade"
      onRequestClose={onCancel}
    >
      <ThemedView variant="dark" style={styles.overlay}>
        <ThemedView variant="primary" style={styles.container}>
          <ThemedText
            variant="light"
            size="large"
            weight="bold"
            style={styles.title}
          >
            {title}
          </ThemedText>

          <ThemedText
            variant="info"
            size="medium"
            style={styles.message}
          >
            {message}
          </ThemedText>

          <ThemedView style={styles.buttonContainer}>
            <Pressable
              style={[styles.button, styles.cancelButton]}
              onPress={onCancel}
              focusable={true}
            >
              <ThemedText
                variant="info"
                size="medium"
                weight="bold"
              >
                {cancelText}
              </ThemedText>
            </Pressable>

            <Pressable
              style={[styles.button, styles.confirmButton]}
              onPress={onConfirm}
              focusable={true}
            >
              <ThemedText
                variant="dark"
                size="medium"
                weight="bold"
              >
                {confirmText}
              </ThemedText>
            </Pressable>
          </ThemedView>
        </ThemedView>
      </ThemedView>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: "rgba(18, 31, 50, 0.9)", // Colors.dark with opacity
  },
  container: {
    width: "40%",
    padding: 40,
    borderRadius: 8,
    alignItems: "center",
  },
  title: {
    marginBottom: 20,
    textAlign: "center",
  },
  message: {
    marginBottom: 32,
    textAlign: "center",
  },
  buttonContainer: {
    flexDirection: "row",
    gap: 20,
  },
  button: {
    paddingVertical: 12,
    paddingHorizontal: 24,
    borderRadius: 6,
    minWidth: 120,
    alignItems: "center",
    transform: [{ scale: 1 }],
  },
  cancelButton: {
    backgroundColor: Colors.primary,
    borderWidth: 2,
    borderColor: Colors.info,
  },
  confirmButton: {
    backgroundColor: Colors.secondary,
  },
});
