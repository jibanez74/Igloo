import { StyleSheet, Pressable, Image, View, ViewStyle } from "react-native";
import { useRouter } from "expo-router";
import { useState } from "react";
import Colors from "@/constants/Colors";
import ThemedText from "@/components/ThemedText";
import type { SimpleMovie } from "@/types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  style?: ViewStyle;
  hasTVPreferredFocus?: boolean;
};

export default function MovieCard({
  movie,
  style,
  hasTVPreferredFocus,
}: MovieCardProps) {
  const [isFocused, setIsFocused] = useState(false);
  const router = useRouter();

  return (
    <Pressable
      style={[styles.card, style, isFocused && styles.cardFocused]}
      focusable={true}
      hasTVPreferredFocus={hasTVPreferredFocus}
      onFocus={() => setIsFocused(true)}
      onBlur={() => setIsFocused(false)}
      onPress={() =>
        router.push({
          pathname: "/movies/[movieID]",
          params: { movieID: movie.ID },
        })
      }
    >
      <Image
        source={{ uri: movie.thumb }}
        style={styles.poster}
        resizeMode='cover'
      />

      <View style={styles.contentContainer}>
        <ThemedText
          variant={isFocused ? "dark" : "light"}
          size='medium'
          weight='bold'
          style={styles.title}
          numberOfLines={2}
        >
          {movie.title}
        </ThemedText>

        <ThemedText
          variant={isFocused ? "dark" : "info"}
          size='small'
          style={styles.year}
        >
          {movie.year}
        </ThemedText>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: Colors.primary,
    borderRadius: 8,
    overflow: "hidden",
    transform: [{ scale: 1.0 }],
  },
  cardFocused: {
    backgroundColor: Colors.secondary,
    transform: [{ scale: 1.1 }],
    zIndex: 1,
  },
  poster: {
    width: "100%",
    aspectRatio: 2 / 3, // Standard movie poster ratio
  },
  contentContainer: {
    padding: 12,
  },
  title: {
    marginBottom: 4,
  },
  year: {
    opacity: 0.8,
  },
});
