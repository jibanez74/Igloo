import { useState, useRef, useEffect } from "react";
import { Platform, StyleSheet, View } from "react-native";
import { VideoRef, Video } from "react-native-video";

type VideoPlayerProps = {
  uri: string;
  thumb: string;
  title: string;
};

export default function VideoPlayer({ uri, thumb, title }: VideoPlayerProps) {
  const videoRef = useRef<VideoRef>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isPaused, setIsPaused] = useState(true);
  const [volume, setVolume] = useState(0.5);

  useEffect(() => {
    const timer = setTimeout(() => {
      setIsPaused(false);
    }, 100);

    return () => clearTimeout(timer);
  }, []);

  useEffect(() => {
    if (Platform.isTV) {
      const handleKeyPress = (event: { keyCode: number }) => {
        switch (event.keyCode) {
          case 19: // DPAD_UP
            setVolume((prev) => Math.min(1, prev + 0.1));
            break;
          case 20: // DPAD_DOWN
            setVolume((prev) => Math.max(0, prev - 0.1));
            break;
          case 23: // SELECT
            setIsPaused(!isPaused);
            break;
          case 127: // MEDIA_PLAY_PAUSE
            setIsPaused(!isPaused);
            break;
          case 128: // MEDIA_STOP
            setIsPaused(true);
            break;
          case 130: // MEDIA_MUTE
            setIsMuted(!isMuted);
            break;
        }
      };

      const subscription =
        Platform.OS === "android"
          ? require("react-native").TVEventHandler.addListener(
              "keyDown",
              handleKeyPress
            )
          : null;

      return () => {
        subscription?.remove();
      };
    }
  }, [isPaused, isMuted]);

  return (
    <View style={styles.container}>
      <Video
        ref={videoRef}
        source={{
          uri,
          type: "mp4",
        }}
        fullscreen={true}
        fullscreenAutorotate={false}
        muted={isMuted}
        paused={isPaused}
        poster={thumb}
        playInBackground={false}
        renderToHardwareTextureAndroid={
          Platform.isTV && Platform.OS === "android"
        }
        volume={volume}
        repeat={false}
        controls={false}
        style={styles.video}
        resizeMode="cover"
        onLoad={() => {
          console.log("video loaded");
          console.log(`your video url is ${uri}`);
        }}
        onError={(err) => {
          console.error(err);
          console.log("an error occurred while loading the video");
        }}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "black",
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
  },
  video: {
    flex: 1,
    width: "100%",
    height: "100%",
  },
});
