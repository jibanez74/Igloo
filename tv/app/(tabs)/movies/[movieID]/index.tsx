import { View, Text } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import api from "@/lib/api";
import type { MovieResponse } from "@/types/Movie";

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams();

  const { data, isPending, isError, error } = useQuery({
    queryKey: ["movie", movieID],
    queryFn: async () => {
      try {
        const { data } = await api.get<MovieResponse>(
          `/auth/movies/${movieID}`
        );

        if (!data.movie) {
          throw new Error("unable to fetch movie");
        }

        return data.movie;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch movie from server");
      }
    },
    refetchOnWindowFocus: false,
  });

  return (
    <View>
      <Text>{movieID}</Text>
    </View>
  );
}
