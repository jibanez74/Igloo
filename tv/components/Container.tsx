import { View } from "react-native";
import type { ViewProps } from "react-native";

type ContainerProps = ViewProps & {
  children: React.ReactNode;
  className?: string;
};

export default function Container({ children, className = "" }: ContainerProps) {
  return (
    <View className={`px-10 max-w-[1920px] mx-auto w-full ${className}`}>
      {children}
    </View>
  );
}
