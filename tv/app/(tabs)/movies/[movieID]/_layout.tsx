import { Stack } from "expo-router";

export default function MovieDetailsLayout() {
  return(
    <Stack>
      <Stack.Screen name="index" options={{ headerShown: false}} />
      <Stack.Screen name="play" options={{ headerShown: false}} />
    </Stack>
  )
}