import { Tabs } from "expo-router";
import { View, Text, Pressable, StyleSheet, Image } from "react-native";
import { useRouter, usePathname } from "expo-router";
import Colors from "@/constants/Colors";

function NavigationHeader() {
  const router = useRouter();
  const currentPath = usePathname();

  const isActive = (path: string) => {
    if (path === "/(tabs)/" && currentPath === "/(tabs)") return true;
    return currentPath.includes(path);
  };

  return (
    <View style={styles.container}>
      {/* Main Navigation */}
      <View style={styles.mainNav}>
        {/* Logo Section */}
        <Pressable
          focusable
          style={({ focused }) => [
            styles.logoContainer,
            focused && styles.focusedBorder,
          ]}
          onPress={() => router.push("/(tabs)/")}
        >
          <Text style={styles.logoText}>IGLOO</Text>
        </Pressable>

        {/* Primary Navigation Items */}
        <View style={styles.navItems}>
          <Pressable
            focusable
            style={({ focused }) => [
              styles.navItem,
              isActive("/(tabs)/") && styles.activeItem,
              focused && styles.focusedItem,
            ]}
            onPress={() => router.push("/(tabs)/")}
          >
            <Text style={[
              styles.navText,
              isActive("/(tabs)/") && styles.activeText,
            ]}>
              Home
            </Text>
          </Pressable>

          <Pressable
            focusable
            style={({ focused }) => [
              styles.navItem,
              isActive("/(tabs)/movies") && styles.activeItem,
              focused && styles.focusedItem,
            ]}
            onPress={() => router.push("/(tabs)/movies")}
          >
            <Text style={[
              styles.navText,
              isActive("/(tabs)/movies") && styles.activeText,
            ]}>
              Movies
            </Text>
          </Pressable>

          <Pressable
            focusable
            style={({ focused }) => [
              styles.navItem,
              isActive("/(tabs)/tv-shows") && styles.activeItem,
              focused && styles.focusedItem,
            ]}
            onPress={() => router.push("/(tabs)/tv-shows")}
          >
            <Text style={[
              styles.navText,
              isActive("/(tabs)/tv-shows") && styles.activeText,
            ]}>
              TV Shows
            </Text>
          </Pressable>
        </View>
      </View>

      {/* Secondary Navigation (Right Side) */}
      <View style={styles.secondaryNav}>
        <Pressable
          focusable
          style={({ focused }) => [
            styles.navItem,
            isActive("/(tabs)/search") && styles.activeItem,
            focused && styles.focusedItem,
          ]}
          // onPress={() => router.push("/(tabs)/search")}
        >
          <Text style={styles.navText}>Search</Text>
        </Pressable>

        <Pressable
          focusable
          style={({ focused }) => [
            styles.navItem,
            isActive("/(tabs)/settings") && styles.activeItem,
            focused && styles.focusedItem,
          ]}
          // onPress={() => router.push("/(tabs)/settings")}
        >
          <Text style={styles.navText}>Settings</Text>
        </Pressable>
      </View>
    </View>
  );
}

export default function TabLayout() {
  return (
    <Tabs
      screenOptions={{
        headerShown: true,
        header: () => <NavigationHeader />,
        tabBarStyle: { display: "none" },
      }}
    >
      <Tabs.Screen name="index" />
      <Tabs.Screen name="movies" />
      <Tabs.Screen name="tv-shows" />
      <Tabs.Screen name="search" />
      <Tabs.Screen name="settings" />
    </Tabs>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    backgroundColor: Colors.primary,
    paddingVertical: 16,
    paddingHorizontal: 40,
    borderBottomWidth: 1,
    borderBottomColor: Colors.borderColor,
  },
  mainNav: {
    flexDirection: "row",
    alignItems: "center",
    flex: 1,
  },
  logoContainer: {
    marginRight: 48,
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 4,
  },
  logoText: {
    color: Colors.textPrimary,
    fontSize: 28,
    fontWeight: "bold",
  },
  navItems: {
    flexDirection: "row",
    gap: 32,
  },
  navItem: {
    paddingVertical: 8,
    paddingHorizontal: 16,
    borderRadius: 4,
    borderWidth: 2,
    borderColor: "transparent",
  },
  navText: {
    color: Colors.textSecondary,
    fontSize: 20,
    fontWeight: "500",
  },
  activeItem: {
    backgroundColor: Colors.secondary,
  },
  activeText: {
    color: Colors.textPrimary,
  },
  focusedItem: {
    borderColor: Colors.accent,
    transform: [{ scale: 1.1 }],
  },
  focusedBorder: {
    borderWidth: 2,
    borderColor: Colors.accent,
  },
  secondaryNav: {
    flexDirection: "row",
    gap: 24,
  },
});
