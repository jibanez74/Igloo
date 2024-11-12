import { useState } from "react";
import { View, StyleSheet, Image, Pressable } from "react-native";
import { Colors } from "@/constants/Colors";
import { ThemedView } from "./ThemedView";
import { ThemedText } from "./ThemedText";
import type { Studio } from "@/types/Studio";

type StudioGridProps = {
  studios: Studio[];
};

export default function StudioGrid({ studios }: StudioGridProps) {
  const [focusedId, setFocusedId] = useState<number | null>(null);
  const [loadedImages, setLoadedImages] = useState<Record<string, boolean>>({});

  const handleImageLoad = (id: number) => {
    setLoadedImages(prev => ({
      ...prev,
      [id]: true,
    }));
  };

  return (
    <View style={styles.container}>
      {studios.map((studio, index) => (
        <Pressable
          key={studio.ID}
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
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 20,
  },
  pressable: {
    transform: [{ scale: 1 }],
  },
  pressableFocused: {
    transform: [{ scale: 1.05 }],
  },
  studioCard: {
    width: 200,
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
