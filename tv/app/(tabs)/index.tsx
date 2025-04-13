import { View, StyleSheet } from "react-native";
import { dark } from "@/constants/Colors";
import LatestMovies from "@/components/LatestMovies";

export default function HomeScreen() {
  return (
    <View style={styles.container}>
      <LatestMovies />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: dark,
  },
});
