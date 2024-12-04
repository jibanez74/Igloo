import { View, Text } from "react-native";
import { useLocalSearchParams } from "expo-router";

export default function PlayMovieScreen() {
  const { movieID } = useLocalSearchParams();

  return (
    <View>
      <Text>You will now watch movie {movieID}</Text>
    </View>
  );
}
