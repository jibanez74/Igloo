import { View, Text, Image, ScrollView } from "react-native";
import getImgSrc from "@/lib/getImgSrc";
import type { Cast } from "@/types/Cast";

type CastGridProps = {
  cast: Cast[];
  maxDisplay?: number;
};

export default function CastGrid({ cast, maxDisplay = 6 }: CastGridProps) {
  return (
    <View className='mb-6'>
      <Text className='text-light text-2xl font-bold mb-4'>Cast</Text>

      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        className='flex-row'
      >
        {cast.slice(0, maxDisplay).map(castMember => (
          <View key={castMember.ID} className='mr-6 items-center w-[120px]'>
            {/* Artist Image */}
            <View className='w-20 h-20 rounded-full overflow-hidden mb-2 bg-primary/20'>
              {castMember.artist.thumb && (
                <Image
                  source={{ uri: getImgSrc(castMember.artist.thumb) }}
                  className='w-full h-full'
                  resizeMode='cover'
                />
              )}
            </View>

            {/* Artist Name */}
            <Text
              className='text-light text-lg text-center mb-1'
              numberOfLines={1}
            >
              {castMember.artist.name}
            </Text>

            {/* Character Name */}
            <Text className='text-info text-base text-center' numberOfLines={1}>
              {castMember.character}
            </Text>
          </View>
        ))}
      </ScrollView>
    </View>
  );
}
