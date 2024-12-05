import { View, Text, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import api from "@/lib/api";
import getError from "@/lib/getError";
import MoviesGrid from "@/components/MoviesGrid";
import { dark, light, info } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";
import type { MoviesResponse } from "@/types/Movie";

export default function MoviesScreen() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["movies"],
    queryFn: async () => {
      try {
        const { data } = await api.get<MoviesResponse>("/auth/movies/all");

        if (!data.movies) {
          throw new Error("no movies were returned by the server");
        }

        return data.movies;
      } catch (err) {
        throw new Error(getError(err));
      }
    },
  });

  if (isPending) {
    return (
      <View style={styles.container}>
        <Text style={styles.loadingText}>Loading movies...</Text>
      </View>
    );
  }

  if (isError) {
    return (
      <View style={styles.container}>
        <Text style={styles.errorText}>
          {error instanceof Error ? error.message : "An error occurred"}
        </Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Movies</Text>
      <MoviesGrid movies={data} />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: Layout.spacing.xl,
    backgroundColor: dark,
  },
  title: {
    fontSize: Layout.card.titleSize * 1.5,
    fontWeight: "bold",
    color: light,
    marginBottom: Layout.spacing.lg,
  },
  loadingText: {
    fontSize: Layout.card.titleSize,
    color: info,
  },
  errorText: {
    fontSize: Layout.card.titleSize,
    color: info,
  },
});
