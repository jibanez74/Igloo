import { View } from "react-native";
import Container from "@/components/Container";
import LatestMovies from "@/components/LatestMovies";

export default function HomeScreen() {
  return (
    <View className='flex-1 bg-dark'>
      <Container>
        {/* Latest Movies Section */}
        <LatestMovies />

        {/* Future sections will use TV-specific navigation */}
        {/* Example:
        <PopularMovies />
        <ContinueWatching />
        <Recommendations />
        */}
      </Container>
    </View>
  );
}
