import { View, Text, FlatList } from "react-native";
import { useQuery } from "@tanstack/react-query";
import API_URL from "@/constants/Backend";
import MovieCard from "@/components/MovieCard";
import type { SimpleMovie } from "@/types/Movie";

export default function HomeScreen() {
  const { isPending, error, isError, data } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async (): Promise<SimpleMovie[]> => {
      try {
        const res = await fetch(`${API_URL}/movies/latest`);
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data.movies;
      } catch (err) {
        console.error(err);
        return [];
      }
    },
  });

  if (isError) {
    console.error(error);
  }

  return (
    <View>
      <Text>Home Screen</Text>

      {isPending && <Text>Loading...</Text>}

      {data && (
        <FlatList
          data={data}
          horizontal
          showsHorizontalScrollIndicator={false}
          keyExtractor={(item) => item.id.toString()}
          renderItem={({ item, index }) => <MovieCard index={index} movie={item} />}
        />
      )}
    </View>
  );
}
