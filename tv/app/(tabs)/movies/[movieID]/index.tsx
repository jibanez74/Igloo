import { View, Text } from "react-native";
import { useLocalSearchParams } from "expo-router";

export default function MovieDetailsScreen() {
  const { movieID } = useLocalSearchParams();

  return (
    <View>
      <Text>Your movie id is {movieID}</Text>
    </View>
  );
}
