import { View, Text, Button } from "react-native";
import { Link, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import type { Movie } from "@/types/Movie";
import API_URL from "@/constants/Backend";

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams();

  const {
    isPending,
    data: movie,
    isError,
    error,
  } = useQuery({
    queryKey: ["movie", movieID],
    queryFn: async (): Promise<Movie> => {
      try {
        const res = await fetch(`${API_URL}/movies/${movieID}/details`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data.movie;
      } catch (err) {
        console.error(err);
        throw new Error("unable to fetch movie");
      }
    },
  });

  if (isError) {
    console.error(error);
  }

  return (
    <View>
      <Text>Your movie id is {movieID}</Text>

      {isPending && <Text>Loading...</Text>}

      {movie && (
        <View>
          <Text>{movie.title}</Text>

          <Text>{movie.summary}</Text>

          <Text>
            Container - {movie.container}
          </Text>

          <Text>
            Content Type - {movie.content_type}
          </Text>

          <Link
            href={{
              pathname: "/(tabs)/movies/[movieID]/play",
              params: { movieID: movie.id.toString() },
            }}
            asChild
          >
            <Button title="Play Movie" />
          </Link>
        </View>
      )}
    </View>
  );
}
