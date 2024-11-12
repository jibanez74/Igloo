import { ActivityIndicator, StyleSheet, View } from "react-native";
import { Colors } from "@/constants/Colors";
import { ThemedText } from "@/components/ThemedText";
import { ThemedView } from "@/components/ThemedView";

interface SpinnerProps {
  text?: string;
}

export default function Spinner({ text }: SpinnerProps) {
  return (
    <ThemedView variant='dark' style={styles.container}>
      <View style={styles.content}>
        <ActivityIndicator size='large' color={Colors.secondary} />

        {text && (
          <ThemedText variant='light' size='large' style={styles.text}>
            {text}
          </ThemedText>
        )}
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
  },
  content: {
    alignItems: "center",
    gap: 16,
  },
  text: {
    textAlign: "center",
  },
});
