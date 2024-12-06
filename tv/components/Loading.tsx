import { View, Text, StyleSheet, ActivityIndicator } from "react-native";
import { light, info } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";

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
        <ActivityIndicator
          size={Layout.isFHD ? "large" : "small"}
          color={info}
          style={styles.spinner}
        />
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
    padding: Layout.spacing.xl,
  },
  spinner: {
    marginBottom: Layout.spacing.lg,
    transform: [{ scale: Layout.isFHD ? 1.5 : 1 }],
  },
  message: {
    fontSize: Layout.card.titleSize,
    color: light,
    textAlign: "center",
  },
});
