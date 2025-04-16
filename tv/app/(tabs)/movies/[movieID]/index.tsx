import { Button, View, Text } from "react-native";
import { useRouter, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import API_URL from "@/constants/Backend";
import type { Movie } from "@/types/Movie";
import Spinner from "@/components/Spinner";

export default function MovieDetailsScreen() {
  const router = useRouter();
  const { movieID } = useLocalSearchParams();

  const {
    isPending,
    isError,
    error,
    data: movie,
  } = useQuery({
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

  if (!movie) {
    return null;
  }

  return (
    <View>
      <Text>Your movie id is {movieID}</Text>

      <Text>{movie.title}</Text>

      <Button
        title={`play movie ${movie.title}`}
        onPress={() =>
          router.navigate({
            pathname: "/(tabs)/movies/[movieID]/play",
            params: {
              movieID: movie.id.toString(),
            }
          })
        }
      />
    </View>
  );
}
