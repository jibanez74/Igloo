import { View } from "react-native";
import Container from "@/components/Container";
import LatestMovies from "@/components/LatestMovies";

export default function HomeScreen() {
  return (
    <View className='flex-1 bg-dark'>
      <Container>
        <LatestMovies />
      </Container>
    </View>
  );
}
