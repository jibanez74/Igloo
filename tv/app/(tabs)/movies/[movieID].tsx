import { View, Text, StyleSheet } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { dark, light } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams();

  return (
    <View style={styles.container}>
      <Text style={styles.text}>Movie ID: {movieID}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: dark,
    padding: Layout.spacing.xl,
  },
  text: {
    color: light,
    fontSize: Layout.card.titleSize,
  },
}); 