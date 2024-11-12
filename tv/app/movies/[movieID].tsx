import { useState } from "react";
import { View, StyleSheet, Image, Pressable, ScrollView } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import { Colors } from "@/constants/Colors";
import { ThemedView } from "@/components/ThemedView";
import { ThemedText } from "@/components/ThemedText";
import Spinner from "@/components/Spinner";
import Alert from "@/components/Alert";
import CastGrid from "@/components/CastGrid";
import GenreList from "@/components/GenreList";
import StudioGrid from "@/components/StudioGrid";
import api from "@/lib/api";
import type { Movie } from "@/types/Movie";
import formatDate from "@/lib/formatDate";

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams();
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
        throw new Error("faile to send request");
      }
    },
  });

  if (isLoading) return <Spinner text='Loading movie details...' />;

  if (isError) {
    return (
      <Alert
        visible={true}
        title='Error'
        message={error.message}
        onConfirm={() => setShowError(false)}
        onCancel={() => setShowError(false)}
      />
    );
  }

  return (
    <ScrollView style={styles.scrollView}>
      <ThemedView variant='dark' style={styles.container}>
        {/* Backdrop Image */}
        <View style={styles.backdropContainer}>
          <Image
            source={{ uri: movie.art }}
            style={styles.backdropImage}
            resizeMode='cover'
          />
          <View style={styles.backdropOverlay} />
        </View>

        <View style={styles.content}>
          {/* Main Info Section */}
          <View style={styles.mainSection}>
            <Image
              source={{ uri: movie.thumb }}
              style={styles.poster}
              resizeMode='cover'
            />

            <View style={styles.infoSection}>
              <ThemedText variant='light' size='xlarge' weight='bold'>
                {movie.title}
              </ThemedText>

              <View style={styles.metadataRow}>
                <ThemedText variant='info' size='medium'>
                  {formatDate(movie.releaseDate)} • {movie.runTime} •{" "}
                  {movie.contentRating}
                </ThemedText>
              </View>

              {movie.tagLine && (
                <ThemedText
                  variant='secondary'
                  size='large'
                  style={styles.tagline}
                >
                  "{movie.tagLine}"
                </ThemedText>
              )}

              <GenreList genres={movie.genres} />

              <ThemedText variant='info' size='medium' style={styles.summary}>
                {movie.summary}
              </ThemedText>

              {/* Ratings Section */}
              <View style={styles.ratingsSection}>
                <ThemedText variant='light' size='large' weight='bold'>
                  Audience Rating: {movie.audienceRating}
                </ThemedText>
              </View>

              {/* Action Buttons */}
              <View style={styles.buttonGroup}>
                <Pressable
                  style={[styles.button, styles.primaryButton]}
                  focusable={true}
                  hasTVPreferredFocus={true}
                >
                  <ThemedText variant='dark' size='large' weight='bold'>
                    ▶ Play
                  </ThemedText>
                </Pressable>

                <Pressable
                  style={[styles.button, styles.secondaryButton]}
                  focusable={true}
                >
                  <ThemedText variant='light' size='large' weight='bold'>
                    ♥ Like
                  </ThemedText>
                </Pressable>

                <Pressable
                  style={[styles.button, styles.secondaryButton]}
                  focusable={true}
                >
                  <ThemedText variant='light' size='large' weight='bold'>
                    • • • More
                  </ThemedText>
                </Pressable>
              </View>
            </View>
          </View>

          {/* Additional Details */}
          <View style={styles.detailsSection}>
            <View style={styles.financialDetails}>
              <View style={styles.detailItem}>
                <ThemedText variant='info' size='medium'>
                  Budget
                </ThemedText>
                <ThemedText variant='light' size='medium'>
                  ${movie.budget?.toLocaleString()}
                </ThemedText>
              </View>
              <View style={styles.detailItem}>
                <ThemedText variant='info' size='medium'>
                  Revenue
                </ThemedText>
                <ThemedText variant='light' size='medium'>
                  ${movie.revenue?.toLocaleString()}
                </ThemedText>
              </View>
            </View>
          </View>

          {/* Cast Section */}
          {movie.castList && (
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
          )}

          {/* Studios Section */}
          {movie.studios && (
            <View style={styles.section}>
              <ThemedText
                variant='light'
                size='large'
                weight='bold'
                style={styles.sectionTitle}
              >
                Studios
              </ThemedText>
              <StudioGrid studios={movie.studios} />
            </View>
          )}
        </View>
      </ThemedView>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  scrollView: {
    flex: 1,
    backgroundColor: Colors.dark,
  },
  container: {
    flex: 1,
  },
  backdropContainer: {
    height: 400,
    position: "relative",
  },
  backdropImage: {
    width: "100%",
    height: "100%",
  },
  backdropOverlay: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: `${Colors.dark}99`,
  },
  content: {
    flex: 1,
    padding: 40,
  },
  mainSection: {
    flexDirection: "row",
    gap: 40,
    marginTop: -100,
  },
  poster: {
    width: 300,
    aspectRatio: 2 / 3,
    borderRadius: 8,
  },
  infoSection: {
    flex: 1,
    gap: 20,
  },
  metadataRow: {
    flexDirection: "row",
    gap: 8,
  },
  tagline: {
    fontStyle: "italic",
  },
  summary: {
    lineHeight: 24,
  },
  ratingsSection: {
    marginVertical: 16,
  },
  buttonGroup: {
    flexDirection: "row",
    gap: 20,
  },
  button: {
    paddingVertical: 16,
    paddingHorizontal: 32,
    borderRadius: 8,
    justifyContent: "center",
    alignItems: "center",
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
  financialDetails: {
    flexDirection: "row",
    gap: 40,
  },
  detailItem: {
    gap: 8,
  },
  section: {
    marginTop: 40,
  },
  sectionTitle: {
    marginBottom: 20,
  },
});
