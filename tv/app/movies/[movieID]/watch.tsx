import { View, Text } from "react-native";
import { useLocalSearchParams } from "expo-router";
import VideoPlayer from "@/components/VideoPlayer";

export default function WatchMovie() {
  const { movieID } = useLocalSearchParams();

  const videoUrl = `https://swifty.hare-crocodile.ts.net/api/v1/auth/movies/stream/direct/${movieID}`;

  return (
    <View>
      <VideoPlayer uri={videoUrl} />
    </View>
  );
}
