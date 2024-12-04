import { StyleSheet, View, Text } from "react-native";
import { dark } from "@/constants/Colors";

export default function MoviesScreen() {
  return (
    <View>
      <Text>Movies</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: dark,
  },
});
