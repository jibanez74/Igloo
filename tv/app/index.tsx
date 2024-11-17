import { View } from "react-native";
import LatestMovies from "@/components/LatestMovies";
import Container from "@/components/Container";

export default function HomeScreen() {
  return (
    <View className='flex-1 bg-dark'>
      <Container>
        <LatestMovies />
        {/* Other sections */}
      </Container>
    </View>
  );
}
