import { useState, useRef, useEffect } from "react";
import {
  Platform,
  useTVEventHandler,
  View,
  StyleSheet,
  Text,
} from "react-native";
import Video, { OnBufferData, VideoRef } from "react-native-video";

type TvVideoPlayerProps = {
  title: string;
  thumb: string;
  videoUri: string;
};

const defaultMaxBitRate = 0;

const defaultBufferConfig = {
  minBufferMs: 10000, // 10s
  maxBufferMs: 30000,
  bufferForPlaybackMs: 2000,
  bufferForPlaybackAfterRebufferMs: 3000,
};

export default function TvVideoPlayer({
  title,
  thumb,
  videoUri,
}: TvVideoPlayerProps) {
  const [isMuted, setIsMuted] = useState(false);
  const [isPaused, setIsPaused] = useState(true);
  const [isBuffering, setIsBuffering] = useState(true);
  const [currentTime, setCurrentTime] = useState(0);
  const [volume, setVolume] = useState(0.5);

  return (
    <View style={styles.container}>
      <Video
        controls={Platform.OS === "ios"}
        fullscreen
        fullscreenAutorotate={false}
        fullscreenOrientation="landscape"
        enterPictureInPictureOnLeave={false}
        maxBitRate={defaultMaxBitRate}
        muted={isMuted}
        onBuffer={(b: OnBufferData) => setIsBuffering(b.isBuffering)}
        onError={err => console.error(err)}
        onProgress={({ currentTime: t }) => setCurrentTime(t)}
        onReadyForDisplay={() => {
          setIsPaused(false);
        }}
        playInBackground={false}
        poster={thumb}
        paused={isPaused}
        renderToHardwareTextureAndroid={Platform.isTV && Platform.OS === "android"}
        resizeMode="contain"
        style={styles.video}
        source={{
          uri: videoUri,
          isNetwork: true,
          headers: {
            Range: "bytes=0-",
          },
        }}
        volume={volume}
        bufferConfig={defaultBufferConfig}
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
  errorContainer: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: "rgba(0,0,0,0.7)",
  },
  errorText: {
    color: "white",
    fontSize: 16,
    textAlign: "center",
    padding: 20,
  },
});
