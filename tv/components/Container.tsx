import { View } from "react-native";

export default function Container({ children }: { children: React.ReactNode }) {
  return (
    <View className="px-10 py-8 max-w-[1920px] mx-auto w-full">
      {children}
    </View>
  );
}
