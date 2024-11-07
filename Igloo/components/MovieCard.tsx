import {
  StyleSheet,
  Text,
  Pressable,
  Image,
  View,
  ViewStyle,
} from "react-native";
import { useRouter } from "expo-router";
import { useState } from "react";
import Colors from "@/constants/Colors";
import type { SimpleMovie } from "@/types/Movie";

interface MovieCardProps {
  movie: SimpleMovie;
  style?: ViewStyle;
}

export default function MovieCard({ movie, style }: MovieCardProps) {
  const [isFocused, setIsFocused] = useState(false);
  const router = useRouter();

  return (
    <Pressable
      style={[styles.card, style, isFocused && styles.cardFocused]}
      focusable={true}
      onFocus={() => setIsFocused(true)}
      onBlur={() => setIsFocused(false)}
      onPress={() => router.push(`/(tabs)/movies/${movie.ID}`)}
    >
      <Image
        source={{ uri: movie.thumb }}
        style={styles.poster}
        resizeMode='cover'
      />

      <View style={styles.contentContainer}>
        <Text
          style={[styles.title, isFocused && styles.titleFocused]}
          numberOfLines={2}
        >
          {movie.title}
        </Text>

        <Text style={[styles.year, isFocused && styles.yearFocused]}>
          {movie.year}
        </Text>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: Colors.cardBackground,
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
    fontSize: 18,
    fontWeight: "bold",
    color: Colors.textPrimary,
    marginBottom: 4,
  },
  titleFocused: {
    color: Colors.textPrimary,
  },
  year: {
    fontSize: 14,
    color: Colors.textSecondary,
  },
  yearFocused: {
    color: Colors.textPrimary,
  },
});
