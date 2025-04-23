import {
  Platform,
  useTVEventHandler,
  View,
  StyleSheet,
  Text,
  ActivityIndicator,
} from "react-native";
import Video, {
  OnBufferData,
  BufferingStrategyType,
  VideoRef,
} from "react-native-video";

type TvVideoPlayerProps = {
  title: string;
  thumb: string;
  videoUri: string;
  container: string;
};

export default function TvVideoPlayer({
  title,
  thumb,
  videoUri,
  container,
}: TvVideoPlayerProps) {
  console.log("component rendering");

  return (
    <View style={styles.container}>
      <Video
        bufferingStrategy={BufferingStrategyType.DEPENDING_ON_MEMORY}
        controls={Platform.OS === "ios"}
        fullscreen
        fullscreenAutorotate={false}
        fullscreenOrientation="landscape"
        enterPictureInPictureOnLeave={false}
        muted={false}
        onError={(err) => console.error(err)}
        onBuffer={(b: OnBufferData) => console.log("video is buffering...")}
        playInBackground={false}
        poster={thumb}
        paused={false}
        renderToHardwareTextureAndroid={
          Platform.isTV && Platform.OS === "android"
        }
        resizeMode="contain"
        style={styles.video}
        source={{
          uri: videoUri,
          isNetwork: true,
          type: container,
        }}
        volume={1}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "black",
  },
  video: {
    flex: 1,
    width: "100%",
    height: "100%",
  },
  overlay: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: "rgba(0, 0, 0, 0.7)",
  },
  overlayText: {
    color: "white",
    fontSize: 18,
    marginTop: 10,
    fontWeight: "500",
  },
});
