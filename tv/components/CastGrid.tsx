import { useState } from "react";
import { View, StyleSheet, Image, Dimensions, Pressable } from "react-native";
import { FlashList } from "@shopify/flash-list";
import Colors from "@/constants/Colors";
import ThemedView from "./ThemedView";
import ThemedText from "./ThemedText";
import type { Cast } from "@/types/Cast";

type CastGridProps = {
  cast: Cast[];
};

const { width: SCREEN_WIDTH } = Dimensions.get("window");
const ITEM_GAP = 20;
const ITEM_WIDTH = 180;

export default function CastGrid({ cast }: CastGridProps) {
  const [focusedId, setFocusedId] = useState<number | null>(null);
  const [loadedImages, setLoadedImages] = useState<Record<number, boolean>>({});

  const handleImageLoad = (id: number) => {
    setLoadedImages(prev => ({
      ...prev,
      [id]: true,
    }));
  };

  const renderCastMember = ({
    item: member,
    index,
  }: {
    item: Cast;
    index: number;
  }) => (
    <Pressable
      focusable={true}
      hasTVPreferredFocus={index === 0}
      onFocus={() => setFocusedId(member.ID)}
      onBlur={() => setFocusedId(null)}
      style={({ focused }) => [
        styles.pressable,
        focused && styles.pressableFocused,
      ]}
      accessibilityLabel={`${member.artist.name} as ${member.character}`}
    >
      <ThemedView
        variant='primary'
        style={[styles.castCard, focusedId === member.ID && styles.cardFocused]}
      >
        <View style={styles.imageContainer}>
          {!loadedImages[member.ID] && (
            <ThemedView variant='dark' style={styles.imagePlaceholder}>
              <ThemedText variant='info' size='small'>
                Loading...
              </ThemedText>
            </ThemedView>
          )}
          <Image
            source={{ uri: member.artist.thumb }}
            style={styles.image}
            resizeMode='cover'
            onLoad={() => handleImageLoad(member.ID)}
          />
        </View>

        <View style={styles.textContainer}>
          <ThemedText
            variant='light'
            size='medium'
            weight='bold'
            numberOfLines={1}
            style={styles.name}
          >
            {member.artist.name}
          </ThemedText>

          <ThemedText
            variant='info'
            size='small'
            numberOfLines={1}
            style={styles.role}
          >
            {member.character}
          </ThemedText>
        </View>
      </ThemedView>
    </Pressable>
  );

  return (
    <View style={styles.container}>
      <FlashList
        data={cast}
        renderItem={renderCastMember}
        estimatedItemSize={ITEM_WIDTH}
        horizontal={true}
        showsHorizontalScrollIndicator={false}
        ItemSeparatorComponent={() => <View style={{ width: ITEM_GAP }} />}
        contentContainerStyle={styles.listContent}
        keyExtractor={item => item.ID.toString()}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    height: 360, // Height to accommodate cast card + padding
  },
  listContent: {
    paddingHorizontal: 4,
  },
  pressable: {
    transform: [{ scale: 1 }],
  },
  pressableFocused: {
    transform: [{ scale: 1.05 }],
  },
  castCard: {
    width: ITEM_WIDTH,
    borderRadius: 8,
    overflow: "hidden",
  },
  cardFocused: {
    borderWidth: 2,
    borderColor: Colors.secondary,
  },
  imageContainer: {
    position: "relative",
    width: "100%",
    height: 270,
    backgroundColor: Colors.dark,
  },
  imagePlaceholder: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
  },
  image: {
    width: "100%",
    height: "100%",
  },
  textContainer: {
    padding: 12,
    gap: 4,
  },
  name: {
    width: "100%",
  },
  role: {
    width: "100%",
    opacity: 0.8,
  },
});
