import { View, StyleSheet } from "react-native";
import ThemedText from "./ThemedText";
import type { Genre } from "@/types/Genre";

type GenreListProps = {
  genres: Genre[];
};

export default function GenreList({ genres }: GenreListProps) {
  return (
    <View style={styles.container}>
      <ThemedText variant='info' size='medium'>
        {genres.map(
          (genre, index) =>
            `${genre.tag}${index < genres.length - 1 ? ", " : ""}`
        )}
      </ThemedText>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    flexWrap: "wrap",
  },
});
