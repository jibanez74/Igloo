import { View, StyleSheet } from "react-native";
import { FlashList } from "@shopify/flash-list";
import MovieCard from "./MovieCard";
import { Layout } from "@/constants/Layout";
import type { SimpleMovie } from "@/types/Movie";

export default function MoviesGrid({ movies }: { movies: SimpleMovie[] }) {
  // Calculate number of columns based on screen width
  const numColumns = Math.floor(
    (Layout.window.width - Layout.spacing.xl * 2) /
      (Layout.card.width + Layout.spacing.lg)
  );

  return (
    <View style={styles.container}>
      <FlashList
        data={movies}
        numColumns={numColumns}
        renderItem={({ item, index }) => (
          <View
            style={[
              styles.cardContainer,
              // Remove right margin for last item in row
              (index + 1) % numColumns === 0 && styles.lastInRow,
              // Add bottom margin except for last row
              index < movies.length - numColumns && styles.bottomMargin,
            ]}
          >
            <MovieCard
              movie={item}
              hasTVPreferredFocus={index === 0}
            />
          </View>
        )}
        estimatedItemSize={Layout.card.height + Layout.spacing.lg}
        contentContainerStyle={styles.contentContainer}
        showsVerticalScrollIndicator={false}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  contentContainer: {
    padding: Layout.spacing.xl,
  },
  cardContainer: {
    marginRight: Layout.spacing.lg,
    marginBottom: Layout.spacing.lg,
  },
  lastInRow: {
    marginRight: 0,
  },
  bottomMargin: {
    marginBottom: Layout.spacing.lg,
  },
});
