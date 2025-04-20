import { Tabs } from "expo-router";

export default function TabsLayout() {
  return(
    <Tabs screenOptions={{ tabBarPosition: "top"}}>
      <Tabs.Screen name="index" options={{ headerShown: false}} />
    </Tabs>
  )
}