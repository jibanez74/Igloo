import { useState, useRef } from "react";
import { Platform } from "react-native";
import { VideoRef, Video } from "react-native-video";

type VideoPlayerProps = {
  uri: string;
  thumb: string;
  title: string;
};

export default function VideoPlayer({ uri, thumb, title }: VideoPlayerProps) {
  const videoRef = useRef<VideoRef>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [volume, setVolume] = useState(0.5);

  return (
    <Video
      ref={videoRef}
      source={{ uri }}
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
      controls={true}
    />
  );
}
