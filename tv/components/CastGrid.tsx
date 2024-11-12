import { useState } from "react";
import { View, StyleSheet, Image, ScrollView, Pressable } from "react-native";
import { Colors } from "@/constants/Colors";
import { ThemedView } from "./ThemedView";
import { ThemedText } from "./ThemedText";
import type { Cast } from "@/types/Cast";

type CastGridProps = {
  cast: Cast[];
};

export default function CastGrid({ cast }: CastGridProps) {
  const [focusedId, setFocusedId] = useState<number | null>(null);
  const [loadedImages, setLoadedImages] = useState<Record<string, boolean>>({});

  const handleImageLoad = (id: number) => {
    setLoadedImages(prev => ({
      ...prev,
      [id]: true,
    }));
  };

  return (
    <ScrollView horizontal showsHorizontalScrollIndicator={false}>
      <View style={styles.container}>
        {cast.map((member, index) => (
          <Pressable
            key={member.ID}
            focusable={true}
            hasTVPreferredFocus={index === 0}
            onFocus={() => setFocusedId(member.ID)}
            onBlur={() => setFocusedId(null)}
            style={({ focused }) => [
              styles.pressable,
              focused && styles.pressableFocused,
            ]}
            // accessibilityLabel={`${member.artist.name} as ${member.character}`}
            // accessibilityHint='Press to view cast member details'
            // accessibilityState={{
            //   selected: focusedId === member.ID,
            // }}
          >
            <ThemedView
              variant='primary'
              style={[
                styles.castCard,
                focusedId === member.ID && styles.cardFocused,
              ]}
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
        ))}
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: "row",
    gap: 20,
    paddingVertical: 20,
    paddingHorizontal: 4,
  },
  pressable: {
    transform: [{ scale: 1 }],
  },
  pressableFocused: {
    transform: [{ scale: 1.05 }],
  },
  castCard: {
    width: 180,
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
    backgroundColor: Colors.dark,
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
