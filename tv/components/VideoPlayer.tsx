import { useState } from "react";
import { Platform, useTVEventHandler, View, StyleSheet } from "react-native";
import Video, { OnBufferData, VideoRef } from "react-native-video";

type TvVideoPlayerProps = {
  title: string;
  thumb: string;
  videoUri: string;
};

const defaultMaxBitRate = 40000000;

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
        controls={Platform.OS === "ios"} // Only enable controls on iOS
        fullscreen
        fullscreenAutorotate={false}
        fullscreenOrientation="landscape"
        enterPictureInPictureOnLeave={false}
        maxBitRate={defaultMaxBitRate}
        muted={isMuted}
        onBuffer={(b: OnBufferData) => setIsBuffering(b.isBuffering)}
        onError={(err) => console.error(err)}
        onProgress={({ currentTime: t }) => setCurrentTime(t)}
        onReadyForDisplay={() => setIsPaused(false)}
        playInBackground={false}
        poster={thumb}
        paused={isPaused}
        renderToHardwareTextureAndroid={
          Platform.isTV && Platform.OS === "android"
        }
        resizeMode="contain"
        style={styles.video}
        source={{
          uri: videoUri,
          isNetwork: true,
        }}
        volume={volume}
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
});
