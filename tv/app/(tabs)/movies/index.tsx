import { View, Text, StyleSheet } from "react-native";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import api from "@/lib/api";
import getError from "@/lib/getError";
import MoviesGrid from "@/components/MoviesGrid";
import Alert from "@/components/Alert";
import Loading from "@/components/Loading";
import { dark, light } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";
import type { MoviesResponse } from "@/types/Movie";

export default function MoviesScreen() {
  const [showError, setShowError] = useState(true);

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
        <Loading message="Loading movies..." />
      </View>
    );
  }

  if (isError && showError) {
    return (
      <View style={styles.container}>
        <Alert
          type="error"
          message={error instanceof Error ? error.message : "An error occurred"}
          onDismiss={() => setShowError(false)}
        />
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Movies</Text>
      <MoviesGrid movies={data || []} />
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
});
