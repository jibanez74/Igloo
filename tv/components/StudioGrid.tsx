import { useState } from "react";
import { View, StyleSheet, Image, Dimensions, Pressable } from "react-native";
import { FlashList } from "@shopify/flash-list";
import Colors from "@/constants/Colors";
import ThemedView from "./ThemedView";
import ThemedText from "./ThemedText";
import type { Studio } from "@/types/Studio";

type StudioGridProps = {
  studios: Studio[];
};

const { width: SCREEN_WIDTH } = Dimensions.get("window");
const ITEM_GAP = 20;
const ITEM_WIDTH = 200;

export default function StudioGrid({ studios }: StudioGridProps) {
  const [focusedId, setFocusedId] = useState<number | null>(null);
  const [loadedImages, setLoadedImages] = useState<Record<number, boolean>>({});

  const handleImageLoad = (id: number) => {
    setLoadedImages(prev => ({
      ...prev,
      [id]: true,
    }));
  };

  const renderStudio = ({
    item: studio,
    index,
  }: {
    item: Studio;
    index: number;
  }) => (
    <Pressable
      focusable={true}
      hasTVPreferredFocus={index === 0}
      onFocus={() => setFocusedId(studio.ID)}
      onBlur={() => setFocusedId(null)}
      style={({ focused }) => [
        styles.pressable,
        focused && styles.pressableFocused,
      ]}
    >
      <ThemedView
        variant='primary'
        style={[
          styles.studioCard,
          focusedId === studio.ID && styles.cardFocused,
        ]}
      >
        <View style={styles.imageContainer}>
          {!loadedImages[studio.ID] && (
            <ThemedView variant='dark' style={styles.imagePlaceholder}>
              <ThemedText variant='info' size='small'>
                Loading...
              </ThemedText>
            </ThemedView>
          )}
          <Image
            source={{ uri: studio.logo }}
            style={styles.logo}
            resizeMode='contain'
            onLoad={() => handleImageLoad(studio.ID)}
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
            {studio.name}
          </ThemedText>
        </View>
      </ThemedView>
    </Pressable>
  );

  return (
    <View style={styles.container}>
      <FlashList
        data={studios}
        renderItem={renderStudio}
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
    height: 180, // Fixed height for horizontal list
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
  studioCard: {
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
    height: 100,
    backgroundColor: Colors.dark,
    justifyContent: "center",
    alignItems: "center",
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
  logo: {
    width: "80%",
    height: "80%",
  },
  textContainer: {
    padding: 12,
  },
  name: {
    textAlign: "center",
  },
});
