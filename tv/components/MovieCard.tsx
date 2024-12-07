import { useRef, useState, useEffect } from "react";
import {
  Image,
  Pressable,
  View,
  Text,
  StyleSheet,
  Animated,
} from "react-native";
import { Link } from "expo-router";
import { primary, dark, light, info, secondary } from "@/constants/Colors";
import getImgSrc from "@/lib/getImgSrc";
import type { SimpleMovie } from "@/types/Movie";

const POSTER_WIDTH = 200; // Base width for 1080p TV
const POSTER_HEIGHT = 300; // Maintain 2:3 aspect ratio (500:750 = 2:3)

type MovieCardProps = {
  movie: SimpleMovie;
  hasTvFocus?: boolean;
  onFocus?: () => void;
  onBlur?: () => void;
};

export default function MovieCard({
  movie,
  hasTvFocus = false,
  onFocus,
  onBlur,
}: MovieCardProps) {
  const [isFocused, setIsFocused] = useState(false);
  const scaleValue = useRef(new Animated.Value(1)).current;

  useEffect(() => {
    Animated.spring(scaleValue, {
      toValue: hasTvFocus ? 1.1 : 1,
      useNativeDriver: true,
    }).start();
  }, [hasTvFocus]);

  const handleFocus = () => {
    onFocus?.();
    setIsFocused(true);
  };

  const handleBlur = () => {
    onBlur?.();
    setIsFocused(false);
  };

  return (
    <Link
      href={{
        pathname: "/(tabs)/movies/[movieID]",
        params: { movieID: movie.ID },
      }}
      asChild
    >
      <Pressable
        accessibilityLabel={`Movie: ${movie.title}, released in ${movie.year}`}
        focusable={true}
        hasTVPreferredFocus={hasTvFocus}
        onFocus={handleFocus}
        onBlur={handleBlur}
        style={({ pressed }) => [styles.pressable, pressed && { opacity: 0.9 }]}
      >
        <Animated.View
          style={[
            styles.card,
            {
              transform: [{ scale: scaleValue }],
            },
          ]}
        >
          <View style={styles.posterContainer}>
            <Image
              source={{ uri: getImgSrc(movie.thumb) }}
              style={styles.poster}
              resizeMode='cover'
            />
          </View>
          <View style={styles.content}>
            <Text style={styles.title} numberOfLines={1} allowFontScaling>
              {movie.title}
            </Text>
            <Text style={styles.year}>{movie.year}</Text>
          </View>
          {isFocused && <View style={styles.focusBorder} />}
        </Animated.View>
      </Pressable>
    </Link>
  );
}

const styles = StyleSheet.create({
  pressable: {
    margin: 8,
  },
  card: {
    width: POSTER_WIDTH,
    backgroundColor: `${primary}33`,
    borderRadius: 8,
    overflow: "hidden",
  },
  posterContainer: {
    width: POSTER_WIDTH,
    height: POSTER_HEIGHT,
    backgroundColor: dark,
  },
  poster: {
    width: "100%",
    height: "100%",
  },
  content: {
    padding: 12,
  },
  title: {
    fontSize: 16,
    fontWeight: "bold",
    color: light,
    marginBottom: 4,
  },
  year: {
    fontSize: 14,
    color: info,
  },
  focusBorder: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    borderWidth: 3,
    borderColor: secondary,
    borderRadius: 8,
  },
});
