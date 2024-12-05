import { Stack } from "expo-router";

export default function MoviesLayout() {
  return (
    <Stack>
      <Stack.Screen name='index' options={{ headerShown: false }} />
      <Stack.Screen name='[movieID]/index' />
      <Stack.Screen name='[movieID]/play' />
    </Stack>
  );
}
