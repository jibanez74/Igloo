import { useRef, useState } from "react";
import { View, StyleSheet } from "react-native";
import Video, {
  VideoRef,
  OnLoadData,
  OnProgressData,
  OnBufferData,
} from "react-native-video";
import Colors from "@/constants/Colors";
import ThemedView from "./ThemedView";
import ThemedText from "./ThemedText";

type VideoPlayerProps = {
  uri: string;
};

export default function VideoPlayer({ uri }: VideoPlayerProps) {
  const videoRef = useRef<VideoRef>(null);
  const [isBuffering, setIsBuffering] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const handleLoad = (data: OnLoadData) => {
    console.log("Video loaded:", data);
    console.log("Audio tracks:", data.audioTracks);
    setIsBuffering(false);
  };

  const handleProgress = (data: OnProgressData) => {
    console.log("Progress:", data.currentTime);
  };

  const handleError = (error: any) => {
    console.error("Video Error:", error);
    setError(error.error?.errorString || "Video playback error");
  };

  return (
    <ThemedView variant='dark' style={styles.container}>
      <View style={styles.videoContainer}>
        <Video
          ref={videoRef}
          source={{ uri }}
          style={styles.video}
          // ExoPlayer specific settings
          useTextureView={false}
          controls={true}
          fullscreen={true}
          resizeMode='contain'
          // Buffer configuration
          bufferConfig={{
            minBufferMs: 15000,
            maxBufferMs: 50000,
            bufferForPlaybackMs: 2500,
            bufferForPlaybackAfterRebufferMs: 5000,
          }}
          // Audio configuration for TrueHD
          selectedAudioTrack={{
            type: "index",
            value: 0,
          }}
          audioOutput='hdmi'
          // TV specific
          focusable={true}
          repeat={false}
          playInBackground={false}
          // Performance settings
          progressUpdateInterval={1000}
          // Event handlers
          onLoad={handleLoad}
          onProgress={handleProgress}
          onBuffer={({ isBuffering }) => setIsBuffering(isBuffering)}
          onError={handleError}
        />

        {isBuffering && (
          <ThemedView variant='dark' style={styles.overlay}>
            <ThemedText variant='light' size='large'>
              Loading...
            </ThemedText>
          </ThemedView>
        )}

        {error && (
          <ThemedView variant='dark' style={styles.overlay}>
            <ThemedText variant='light' size='large'>
              {error}
            </ThemedText>
          </ThemedView>
        )}
      </View>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  videoContainer: {
    flex: 1,
    backgroundColor: Colors.dark,
  },
  video: {
    flex: 1,
  },
  overlay: {
    ...StyleSheet.absoluteFillObject,
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: `${Colors.dark}E6`,
  },
});
