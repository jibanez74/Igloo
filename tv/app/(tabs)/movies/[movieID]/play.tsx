import { View, Text } from "react-native";
import { useLocalSearchParams } from "expo-router";

export default function PlayMovie() {
  const { movieID } = useLocalSearchParams();

  return (
    <View>
      <Text>You have requested to watch the movie {movieID}</Text>
    </View>
  );
}
