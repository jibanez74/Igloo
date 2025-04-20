import { useEffect } from "react";
import { TVEventControl, View, StyleSheet } from "react-native";
import { Slot } from "expo-router";
import { useFonts } from "expo-font";
import * as SplashScreen from "expo-splash-screen";
import { QueryClientProvider, QueryClient } from "@tanstack/react-query";
import { dark } from "@/constants/Colors";

SplashScreen.preventAutoHideAsync();

const queryClient = new QueryClient();

export default function MainLayout() {
  const [loaded] = useFonts({
    SpaceMono: require("../assets/fonts/SpaceMono-Regular.ttf"),
  });

  useEffect(() => {
    if (loaded) {
      SplashScreen.hideAsync();
      TVEventControl.enableTVMenuKey();
    }
  }, [loaded]);

  if (!loaded) {
    return null;
  }

  return (
    <QueryClientProvider client={queryClient}>
      <View style={styles.container}>
        <Slot />
      </View>
    </QueryClientProvider>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: dark,
  },
});
