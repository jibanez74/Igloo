import { View, Text, Image, ScrollView } from "react-native";
import getImgSrc from "@/lib/getImgSrc";
import type { Studio } from "@/types/Studio";

type StudioGridProps = {
  studios: Studio[];
  maxDisplay?: number;
};

export default function StudioGrid({
  studios,
  maxDisplay = 6,
}: StudioGridProps) {
  return (
    <View className='mb-6'>
      <Text className='text-light text-2xl font-bold mb-4'>Studios</Text>

      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        className='flex-row'
      >
        {studios.slice(0, maxDisplay).map(studio => (
          <View key={studio.ID} className='mr-6 items-center w-[160px]'>
            {/* Studio Logo */}
            <View className='w-32 h-20 rounded-lg overflow-hidden mb-2 bg-primary/20 items-center justify-center'>
              {studio.logo ? (
                <Image
                  source={{ uri: getImgSrc(studio.logo) }}
                  className='w-full h-full'
                  resizeMode='contain'
                />
              ) : (
                // Fallback when no logo
                <Text className='text-info text-lg text-center'>
                  {studio.name.charAt(0)}
                </Text>
              )}
            </View>

            {/* Studio Name */}
            <Text
              className='text-light text-lg text-center mb-1'
              numberOfLines={1}
            >
              {studio.name}
            </Text>

            {/* Studio Country */}
            {studio.country && (
              <Text
                className='text-info text-base text-center'
                numberOfLines={1}
              >
                {studio.country}
              </Text>
            )}
          </View>
        ))}
      </ScrollView>
    </View>
  );
}
