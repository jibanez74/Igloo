import { StyleSheet, ScrollView } from "react-native";
import { ThemedText } from "@/components/ThemedText";
import { ThemedView } from "@/components/ThemedView";
import LatestMovies from "@/components/LatestMovies";
import { scale } from "react-native-size-matters";

export default function HomeScreen() {
  return (
    <ScrollView style={styles.scrollView}>
      <ThemedView style={styles.container}>
        <ThemedText type="title" style={styles.title}>
          Welcome to Igloo
        </ThemedText>

        <LatestMovies />
      </ThemedView>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  scrollView: {
    flex: 1,
  },
  container: {
    flex: 1,
    padding: scale(20),
  },
  title: {
    marginBottom: scale(20),
  },
});
