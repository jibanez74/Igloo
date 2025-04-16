import { View, Text } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import API_URL from "@/constants/Backend";
import type { Movie } from "@/types/Movie";
import Spinner from "@/components/Spinner";

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams();

  const { isPending, isError, error, data } = useQuery({
    queryKey: ["movie-details", movieID],
    queryFn: async (): Promise<Movie> => {
      try {
        const res = await fetch(`${API_URL}/movies/${movieID}`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data.movie;
      } catch (err) {
        console.error(err);
        throw new Error(
          "a network error occurred while fetching movie details"
        );
      }
    },
  });

  if (isError) {
    console.error(error);
  }

  if (isPending) {
    return <Spinner />;
  }

  return (
    <View>
      <Text>Your movie id is {movieID}</Text>

      <Text>{data?.title}</Text>
    </View>
  );
}
