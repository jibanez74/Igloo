import { useState, useRef } from "react";
import { Platform } from "react-native";
import { VideoRef, Video, type SelectedVideoTrack } from "react-native-video";

type VideoPlayerProps = {
  uri: string;
  thumb: string;
  maxBitRate: number;
  videoTrack: SelectedVideoTrack;
};

type VideoError = {
  error: {
    errorString?: string;
    errorException?: string;
  };
};

export default function VideoPlayer({
  uri,
  thumb,
  maxBitRate,
  videoTrack,
}: VideoPlayerProps) {
  const videoRef = useRef<VideoRef>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [volume, setVolume] = useState(0.5);

  const [error, setError] = useState<string | null>(null);

  const handleError = (error: VideoError) => {
    const errorMessage =
      error.error?.errorString || "An error occurred during playback";
    setError(errorMessage);
    console.error("Video Error:", errorMessage);
  };

  return (
    <Video
      ref={videoRef}
      source={{ uri }}
      fullscreen={true}
      fullscreenAutorotate={false}
      muted={isMuted}
      maxBitRate={maxBitRate}
      onError={handleError}
      paused={isPaused}
      poster={thumb}
      pictureInPicture={false}
      playInBackground={false}
      renderToHardwareTextureAndroid={
        Platform.isTV && Platform.OS === "android"
      }
      selectedVideoTrack={videoTrack}
      volume={volume}
    />
  );
}
