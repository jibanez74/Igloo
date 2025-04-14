import { View, Text, ScrollView } from "react-native";
import { useQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import ErrorWarning from "@/components/ErrorWarning";
import Spinner from "@/components/Spinner";
import { fetchGet } from "@/utils/api";
import type { SimpleMovie } from "@/types/Movie";

type LatestMoviesResponse = {
  movies: SimpleMovie[];
};

export default function LatestMovies() {
  const { isPending, data, isError, error } = useQuery({
    queryKey: ["latest-movies"],
    queryFn: async (): Promise<LatestMoviesResponse> => {
      try {
        const res = await fetchGet("/movies/latest");
        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data;
      } catch (err) {
        console.error(err);
        throw new Error(
          "a network error occurred while fetching latest movies"
        );
      }
    },
  });

  return (
    <View className="py-8">
      <View className="max-w-7xl mx-auto px-4">
        <View className="flex-row items-center justify-between h-10 mb-6">
          <Text className="text-2xl font-bold text-white">
            <Text className="text-yellow-300">Latest Movies</Text>
          </Text>

          <View className="w-5 h-5">
            {isPending && <Spinner size="sm" />}
          </View>
        </View>

        <View className="h-10">
          <ErrorWarning
            error={error?.message || ""}
            isVisible={isError}
          />
        </View>

        <View className="min-h-[200px]">
          {isPending ? (
            <ScrollView horizontal showsHorizontalScrollIndicator={false}>
              <View className="flex-row gap-4">
                {Array(12).fill(null).map((_, index) => (
                  <View
                    key={index}
                    className="w-[200px] h-[300px] rounded-xl bg-blue-950/50"
                  />
                ))}
              </View>
            </ScrollView>
          ) : data?.movies.length === 0 ? (
            <View className="h-full items-center justify-center">
              <Text className="text-blue-200/80">No movies available</Text>
            </View>
          ) : (
            <ScrollView 
              horizontal 
              showsHorizontalScrollIndicator={false}
              contentContainerStyle={{ gap: 16 }}
            >
              {data?.movies.map((movie) => (
                <MovieCard key={movie.id} movie={movie} />
              ))}
            </ScrollView>
          )}
        </View>
      </View>
    </View>
  );
}
