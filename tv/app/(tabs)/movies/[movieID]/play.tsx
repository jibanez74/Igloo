import { View, Text } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import getImgSrc from "@/lib/getImgSrc";
import API_URL from "@/constants/Backend";
import TvVideoPlayer from "@/components/VideoPlayer";
import type { MovieDirectPlayData } from "@/types/Movie";

export default function PlayMovieScreen() {
  const { movieID } = useLocalSearchParams();

  const {
    isPending,
    data,
    isError,
    error,
  } = useQuery({
    queryKey: ["play-movie", movieID],
    queryFn: async (): Promise<MovieDirectPlayData> => {
      try {
        const res = await fetch(`${API_URL}/movies/${movieID}/playback-details`);
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
    refetchOnWindowFocus: false,
  });

  if (isPending) {
    return (
      <View>
        <Text>Movie is loading...</Text>
      </View>
    );
  }

  return (
    <View>
      {data && (
        <TvVideoPlayer
          title={data.title}
          thumb={getImgSrc(data.thumb)}
          videoUri={`${API_URL}/movies/stream${data.file_path}`}
          container={data.container}
        />
      )}
    </View>
  );
}
