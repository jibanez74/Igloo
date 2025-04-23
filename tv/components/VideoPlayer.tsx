import { useState, useRef, useEffect, useCallback } from "react";
import {
  Platform,
  useTVEventHandler,
  View,
  StyleSheet,
  Text,
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
  const videoRef = useRef<VideoRef>(null);
  const [isBuffering, setIsBuffering] = useState(false);

  const handleBuffer = useCallback((data: OnBufferData) => {
    setIsBuffering(data.isBuffering);
  }, []);

  console.log("component rendering");

  return (
    <View style={styles.container}>
      <Video
        ref={videoRef}
        bufferingStrategy={BufferingStrategyType.DEPENDING_ON_MEMORY}
        controls={Platform.OS === "ios"}
        fullscreen
        fullscreenAutorotate={false}
        fullscreenOrientation="landscape"
        enterPictureInPictureOnLeave={false}
        muted={false}
        onError={(err) => console.error(err)}
        onBuffer={handleBuffer}
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
          headers: {
            Range: "bytes=0-",
          },
        }}
        volume={1}
      />
      {isBuffering && (
        <View style={styles.bufferingContainer}>
          <Text style={styles.bufferingText}>Buffering...</Text>
        </View>
      )}
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
  bufferingContainer: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: "rgba(0,0,0,0.7)",
  },
  bufferingText: {
    color: "white",
    fontSize: 16,
    textAlign: "center",
    padding: 20,
  },
});
