import { useState, useRef, useEffect } from "react";
import { Platform, StyleSheet, View, ActivityIndicator } from "react-native";
import { VideoRef, Video } from "react-native-video";
import { ThemedText } from "./ThemedText";


type VideoPlayerProps = {
  uri: string;
  thumb: string;
  title: string;
};

export default function VideoPlayer({ uri, thumb, title }: VideoPlayerProps) {
  const videoRef = useRef<VideoRef>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [volume, setVolume] = useState(1.0);
  const [isBuffering, setIsBuffering] = useState(false);

  useEffect(() => {
    console.log("VideoPlayer mounted with URI:", uri);
    console.log("VideoPlayer configuration:", {
      isMuted,
      isPaused,
      volume,
      thumb,
      title,
    });


  }, [uri, isMuted, isPaused, volume, thumb, title]);

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
          console.log("Video loaded successfully");
          console.log("Video metadata:", {
            uri,
          });
        }}
        onError={(err) => {
          console.error("Video error:", err);
        }}
        onProgress={({ currentTime, playableDuration, seekableDuration }) => {
          console.log("Video progress:", {
            currentTime,
            playableDuration,
            seekableDuration,
          });
        }}
        onBuffer={({ isBuffering }) => {
          console.log("Buffering state:", isBuffering);
          setIsBuffering(isBuffering);
        }}
      />
      {isBuffering && (
        <View style={styles.bufferingContainer}>
          <ActivityIndicator size="large" color="white" />
          <ThemedText type="subtitle" style={styles.bufferingText}>
            Buffering...
          </ThemedText>
        </View>
      )}
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
  errorText: {
    color: "white",
    textAlign: "center",
    marginVertical: 10,
  },
  bufferingContainer: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: "rgba(0, 0, 0, 0.5)",
  },
  bufferingText: {
    color: "white",
    marginTop: 10,
  },
});
