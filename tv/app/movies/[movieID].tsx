import { View, StyleSheet, Image, Pressable } from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Colors } from "@/constants/Colors";
import { ThemedView } from "@/components/ThemedView";
import { ThemedText } from "@/components/ThemedText";
import { Spinner } from "@/components/Spinner";
import api from "@/lib/api";
import type { Movie } from "@/types/Movie";

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams();
  const router = useRouter();

  const { data: movie, isLoading } = useQuery<Movie>({
    queryKey: ["movie", movieID],
    queryFn: async () => {
      const response = await api.get(`/movies/${movieID}`);
      return response.data;
    },
  });

  if (isLoading || !movie) {
    return <Spinner text="Loading movie details..." />;
  }

  return (
    <ThemedView variant="dark" style={styles.container}>
      {/* Background Image with Gradient Overlay */}
      <View style={styles.backgroundContainer}>
        <Image
          source={{ uri: movie.backdrop }}
          style={styles.backgroundImage}
          resizeMode="cover"
        />
        <View style={styles.overlay} />
      </View>

      {/* Content */}
      <View style={styles.content}>
        {/* Movie Poster and Info */}
        <View style={styles.headerSection}>
          <Image
            source={{ uri: movie.poster }}
            style={styles.poster}
            resizeMode="cover"
          />

          <View style={styles.infoContainer}>
            <ThemedText variant="light" size="xlarge" weight="bold">
              {movie.title}
            </ThemedText>

            <ThemedText variant="info" size="large" style={styles.year}>
              {movie.year}
            </ThemedText>

            <ThemedText variant="info" size="medium" style={styles.metadata}>
              {`${movie.duration} • ${movie.rating}`}
            </ThemedText>

            <ThemedText variant="info" size="medium" style={styles.description}>
              {movie.description}
            </ThemedText>

            {/* Action Buttons */}
            <View style={styles.buttonGroup}>
              <Pressable
                style={[styles.button, styles.primaryButton]}
                focusable={true}
                hasTVPreferredFocus={true}
                onPress={() => {/* Handle play */}}
              >
                <ThemedText variant="dark" size="large" weight="bold">
                  ▶ Play
                </ThemedText>
              </Pressable>

              <Pressable
                style={[styles.button, styles.secondaryButton]}
                focusable={true}
                onPress={() => {/* Handle like */}}
              >
                <ThemedText variant="light" size="large" weight="bold">
                  ♥ Like
                </ThemedText>
              </Pressable>

              <Pressable
                style={[styles.button, styles.secondaryButton]}
                focusable={true}
                onPress={() => {/* Handle more options */}}
              >
                <ThemedText variant="light" size="large" weight="bold">
                  • • • More
                </ThemedText>
              </Pressable>
            </View>
          </View>
        </View>

        {/* Additional Details */}
        <View style={styles.detailsSection}>
          <ThemedText variant="light" size="large" weight="bold" style={styles.sectionTitle}>
            Details
          </ThemedText>
          
          {/* Add more movie details based on your Movie type */}
        </View>
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  backgroundContainer: {
    ...StyleSheet.absoluteFillObject,
  },
  backgroundImage: {
    width: "100%",
    height: "100%",
  },
  overlay: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: `${Colors.dark}dd`,
  },
  content: {
    flex: 1,
    padding: 40,
  },
  headerSection: {
    flexDirection: "row",
    gap: 40,
  },
  poster: {
    width: 300,
    aspectRatio: 2/3,
    borderRadius: 8,
  },
  infoContainer: {
    flex: 1,
    gap: 16,
  },
  year: {
    opacity: 0.8,
  },
  metadata: {
    opacity: 0.7,
  },
  description: {
    marginTop: 8,
    lineHeight: 24,
  },
  buttonGroup: {
    flexDirection: "row",
    gap: 20,
    marginTop: 24,
  },
  button: {
    paddingVertical: 16,
    paddingHorizontal: 32,
    borderRadius: 8,
    justifyContent: "center",
    alignItems: "center",
    transform: [{ scale: 1 }],
  },
  primaryButton: {
    backgroundColor: Colors.secondary,
    minWidth: 200,
  },
  secondaryButton: {
    backgroundColor: Colors.primary,
    borderWidth: 2,
    borderColor: Colors.secondary,
    minWidth: 150,
  },
  detailsSection: {
    marginTop: 40,
  },
  sectionTitle: {
    marginBottom: 16,
  },
});
