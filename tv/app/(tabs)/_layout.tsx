import { StyleSheet } from "react-native";
import { Tabs } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import { dark, primary, secondary, info } from "@/constants/Colors";
import { Layout } from "@/constants/Layout";

export default function TabLayout() {
  return (
    <Tabs
      screenOptions={{
        tabBarPosition: "top",
        headerShown: false,
        tabBarStyle: styles.tabBar,
        tabBarActiveTintColor: secondary,
        tabBarInactiveTintColor: info,
        tabBarLabelStyle: styles.tabBarLabel,
        tabBarIconStyle: styles.tabBarIcon,
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: "Home",
          tabBarIcon: ({ focused, color }) => (
            <Ionicons
              name={focused ? "home" : "home-outline"}
              size={Layout.isFHD ? 32 : 24}
              color={color}
            />
          ),
        }}
      />

      <Tabs.Screen
        name="movies"
        options={{
          title: "Movies",
          href: "/movies",
          tabBarIcon: ({ focused, color }) => (
            <Ionicons
              name={focused ? "film" : "film-outline"}
              size={Layout.isFHD ? 32 : 24}
              color={color}
            />
          ),
        }}
      />
    </Tabs>
  );
}

const styles = StyleSheet.create({
  tabBar: {
    backgroundColor: dark,
    borderBottomColor: primary,
    borderBottomWidth: 1,
    height: Layout.isFHD ? 80 : 60,
    paddingVertical: Layout.spacing.sm,
  },
  tabBarLabel: {
    fontSize: Layout.isFHD ? 18 : 14,
    fontWeight: "500",
    marginTop: Layout.spacing.xs,
  },
  tabBarIcon: {
    marginBottom: -Layout.spacing.xs,
  },
});
