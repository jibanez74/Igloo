import { useState } from "react";
import {
  View,
  StyleSheet,
  Image,
  Pressable,
  ImageBackground,
} from "react-native";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import LinearGradient from "react-native-linear-gradient";
import Colors from "@/constants/Colors";
import ThemedView from "@/components/ThemedView";
import ThemedText from "@/components/ThemedText";
import Spinner from "@/components/Spinner";
import Alert from "@/components/Alert";
import CastGrid from "@/components/CastGrid";
import GenreList from "@/components/GenreList";
import formatDate from "@/lib/formatDate";
import api from "@/lib/api";
import type { Movie } from "@/types/Movie";

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams();
  const router = useRouter();

  const [showError, setShowError] = useState(false);

  const {
    data: movie,
    isLoading,
    error,
    isError,
  } = useQuery({
    queryKey: ["movie", movieID],
    queryFn: async () => {
      try {
        const res = await api.get(`/auth/movies/${movieID}`);

        if (!res.data.movie) {
          throw new Error("unable to fetch movie");
        }

        return res.data.movie;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch movie");
      }
    },
  });

  if (isLoading) return <Spinner text='Loading movie details...' />;

  if (isError) {
    return (
      <Alert
        visible={true}
        title='Error'
        message={error?.message || "Failed to load movie"}
        onConfirm={() => setShowError(false)}
        onCancel={() => setShowError(false)}
      />
    );
  }

  return (
    <ThemedView variant='dark' style={styles.container}>
      {/* Hero Section with Backdrop */}
      <ImageBackground
        source={{ uri: movie.art }}
        style={styles.heroSection}
        resizeMode='cover'
      >
        <LinearGradient
          colors={["transparent", `${Colors.dark}CC`, Colors.dark]}
          locations={[0, 0.5, 1.0]}
          style={styles.heroOverlay}
        >
          <View style={styles.heroContent}>
            {/* Movie Title and Basic Info */}
            <ThemedText
              variant='light'
              size='xlarge'
              weight='bold'
              style={styles.title}
            >
              {movie.title}
            </ThemedText>

            <View style={styles.metadataRow}>
              <ThemedText variant='info' size='medium'>
                {movie.releaseDate} • {movie.runTime} • {movie.contentRating}
              </ThemedText>
            </View>

            {/* Action Buttons */}
            <View style={styles.buttonGroup}>
              <Pressable
                style={styles.playButton}
                focusable={true}
                hasTVPreferredFocus={true}
                onPress={() => router.push(`/movies/${movieID}/watch`)}
              >
                <ThemedText variant='dark' size='large' weight='bold'>
                  ▶ Play
                </ThemedText>
              </Pressable>

              <Pressable style={styles.secondaryButton} focusable={true}>
                <ThemedText variant='light' size='large' weight='bold'>
                  + My List
                </ThemedText>
              </Pressable>
            </View>

            {/* Movie Summary */}
            <ThemedText variant='info' size='medium' style={styles.summary}>
              {movie.summary}
            </ThemedText>

            <GenreList genres={movie.genres} />
          </View>
        </LinearGradient>
      </ImageBackground>

      {/* Additional Content */}
      <View style={styles.detailsSection}>
        {/* Cast Section */}
        <View style={styles.section}>
          <ThemedText
            variant='light'
            size='large'
            weight='bold'
            style={styles.sectionTitle}
          >
            Cast
          </ThemedText>
          <CastGrid cast={movie.castList} />
        </View>

        {/* Additional Details */}
        <View style={styles.section}>
          <ThemedText
            variant='light'
            size='large'
            weight='bold'
            style={styles.sectionTitle}
          >
            Details
          </ThemedText>

          <View style={styles.detailsGrid}>
            <View style={styles.detailItem}>
              <ThemedText variant='info' size='medium'>
                Release Date
              </ThemedText>
              <ThemedText variant='light' size='medium'>
                {formatDate(movie.releaseDate)}
              </ThemedText>
            </View>

            <View style={styles.detailItem}>
              <ThemedText variant='info' size='medium'>
                Runtime
              </ThemedText>
              <ThemedText variant='light' size='medium'>
                {movie.runTime}
              </ThemedText>
            </View>

            <View style={styles.detailItem}>
              <ThemedText variant='info' size='medium'>
                Rating
              </ThemedText>
              <ThemedText variant='light' size='medium'>
                {movie.contentRating}
              </ThemedText>
            </View>

            <View style={styles.detailItem}>
              <ThemedText variant='info' size='medium'>
                Audience Score
              </ThemedText>
              <ThemedText variant='light' size='medium'>
                {movie.audienceRating}
              </ThemedText>
            </View>
          </View>
        </View>
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  heroSection: {
    height: 600,
  },
  heroOverlay: {
    flex: 1,
    padding: 40,
    justifyContent: "flex-end",
  },
  heroContent: {
    maxWidth: "60%",
    gap: 20,
  },
  title: {
    fontSize: 48,
  },
  metadataRow: {
    flexDirection: "row",
    gap: 12,
  },
  buttonGroup: {
    flexDirection: "row",
    gap: 20,
    marginTop: 20,
  },
  playButton: {
    backgroundColor: Colors.secondary,
    paddingVertical: 16,
    paddingHorizontal: 40,
    borderRadius: 8,
    transform: [{ scale: 1 }],
  },
  secondaryButton: {
    backgroundColor: Colors.primary,
    paddingVertical: 16,
    paddingHorizontal: 40,
    borderRadius: 8,
    borderWidth: 2,
    borderColor: Colors.secondary,
    transform: [{ scale: 1 }],
  },
  summary: {
    lineHeight: 24,
    marginTop: 20,
  },
  detailsSection: {
    padding: 40,
    gap: 40,
  },
  section: {
    gap: 20,
  },
  sectionTitle: {
    marginBottom: 16,
  },
  detailsGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 40,
  },
  detailItem: {
    minWidth: 200,
    gap: 8,
  },
});
