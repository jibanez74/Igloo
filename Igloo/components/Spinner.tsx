import { StyleSheet, View, ActivityIndicator, Text } from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";
import Colors from "@/constants/Colors";

type SpinnerProps = {
  message?: string;
  size?: "small" | "large";
};

export default function Spinner({
  message = "Loading...",
  size = "large",
}: SpinnerProps) {
  return (
    <SafeAreaView style={styles.safeArea}>
      <View style={styles.container}>
        <ActivityIndicator
          size={size}
          color={Colors.accent}
          style={styles.spinner}
        />
        <Text style={styles.text}>{message}</Text>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safeArea: {
    flex: 1,
    backgroundColor: Colors.primary,
  },
  container: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
  },
  spinner: {
    marginBottom: 16,
  },
  text: {
    color: Colors.textPrimary,
    fontSize: 24,
    textAlign: "center",
  },
});
