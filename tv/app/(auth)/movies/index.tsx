import { View, Text } from "react-native";
import { FlashList } from "@shopify/flash-list";
import { useQuery } from "@tanstack/react-query";
import MovieCard from "@/components/MovieCard";
import api from "@/lib/api";
import type { SimpleMovie } from "@/types/Movie";

type MoviesResponse = {
  movies: SimpleMovie[];
  count: number;
};

export default function MoviesScreen() {
  const { data, error, isError, isPending } = useQuery({
    queryKey: ["movies"],
    queryFn: async () => {
      const { data } = await api.get<MoviesResponse>("/auth/movies/all");
      return data;
    },
    staleTime: 5 * 60 * 1000,
  });

  if (isPending) {
    return (
      <View className="flex-1 bg-dark p-8">
        <Text className="text-info text-2xl">Loading movies...</Text>
      </View>
    );
  }

  if (isError) {
    return (
      <View className="flex-1 bg-dark p-8">
        <Text className="text-danger text-2xl">
          {error?.message || "Error loading movies"}
        </Text>
      </View>
    );
  }

  return (
    <View className="flex-1 bg-dark">
      <Text className="text-light text-4xl font-bold px-8 py-6">
        All Movies ({data?.count || 0})
      </Text>

      <View className="flex-1">
        <FlashList
          data={data?.movies}
          numColumns={6}
          estimatedItemSize={375}
          keyExtractor={item => item.ID.toString()}
          renderItem={({ item, index }) => (
            <View style={{ 
              width: '16.666%',
              paddingHorizontal: 16,
              paddingVertical: 16,
            }}>
              <MovieCard 
                movie={item} 
                hasTVPreferredFocus={index === 0} 
              />
            </View>
          )}
          contentContainerStyle={{
            paddingBottom: 32,
          }}
        />
      </View>
    </View>
  );
}
