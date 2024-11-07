import {
  StyleSheet,
  View,
  FlatList,
  Dimensions,
  ListRenderItem,
} from "react-native";
import MovieCard from "./MovieCard";
import Colors from "@/constants/Colors";
import type { SimpleMovie } from "@/types/Movie";

type MediaGridProps = {
  data: SimpleMovie[];
  onEndReached?: () => void;
};

const COLUMNS = 6;
const SPACING = 20;
const { width: SCREEN_WIDTH } = Dimensions.get("window");
const ITEM_WIDTH = (SCREEN_WIDTH - SPACING * (COLUMNS + 1)) / COLUMNS;
const ITEM_HEIGHT = ITEM_WIDTH * 1.5 + 100; // 1.5 aspect ratio for poster + space for text

export default function MediaGrid({ data, onEndReached }: MediaGridProps) {
  const renderItem: ListRenderItem<SimpleMovie> = ({ item, index }) => {
    return (
      <View style={styles.itemContainer}>
        <MovieCard movie={item} style={{ width: ITEM_WIDTH }} />
      </View>
    );
  };

  return (
    <View style={styles.container}>
      <FlatList
        data={data}
        renderItem={renderItem}
        keyExtractor={item => item.ID}
        numColumns={COLUMNS}
        horizontal={false}
        showsVerticalScrollIndicator={false}
        contentContainerStyle={styles.grid}
        onEndReached={onEndReached}
        onEndReachedThreshold={0.5}
        initialNumToRender={12} // Two rows
        maxToRenderPerBatch={12} // Two rows at a time
        windowSize={5} // Keep 5 rows in memory
        removeClippedSubviews={true} // Optimize memory usage
        getItemLayout={(data, index) => ({
          length: ITEM_HEIGHT,
          offset: ITEM_HEIGHT * Math.floor(index / COLUMNS),
          index,
        })}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.primary,
  },
  grid: {
    padding: SPACING,
  },
  itemContainer: {
    width: ITEM_WIDTH,
    height: ITEM_HEIGHT,
    marginHorizontal: SPACING / 2,
    marginVertical: SPACING,
  },
});
