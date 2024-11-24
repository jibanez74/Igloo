import { useState, useRef, useEffect } from "react";
import { View, Text, Pressable } from "react-native";
import Video, { VideoRef, OnBufferData } from "react-native-video";
import { Ionicons } from "@expo/vector-icons";
import { router } from "expo-router";

type VideoPlayerProps = {
  uri: string;
  onClose?: () => void;
  thumb: string;
  contentType: string; // e.g., 'video/mp4'
  size: number; // File size in bytes
  resolution: number; // e.g., 2160 for 4K, 1080 for Full HD
};

type VideoErrorData = {
  error: {
    errorString?: string;
    errorException?: string;
  };
};

export default function VideoPlayer({
  uri,
  onClose,
  thumb,
  contentType,
  size,
  resolution,
}: VideoPlayerProps) {
  const videoRef = useRef<VideoRef>(null);
  const [isPaused, setIsPaused] = useState(false);
  const [isBuffering, setIsBuffering] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showControls, setShowControls] = useState(true);

  const handleBuffer = (data: OnBufferData) => {
    setIsBuffering(data.isBuffering);
    if (data.isBuffering) {
      console.log("Buffering... Network conditions might require transcoding");
    }
  };

  const handleError = (error: VideoErrorData) => {
    const errorMessage =
      error.error?.errorString || "An error occurred during playback";
    setError(errorMessage);
  };

  const handleBack = () => {
    if (onClose) {
      onClose();
    } else {
      router.back();
    }
  };

  return (
    <View className='flex-1 bg-black'>
      <Video
        ref={videoRef}
        source={{
          uri,
          type: contentType, // Pass content type
          headers: {
            // Optional headers
            "Accept-Ranges": "bytes",
          },
        }}
        paused={isPaused}
        onBuffer={handleBuffer}
        onError={handleError}
        repeat={false}
        controls={true}
        fullscreen={true}
        poster={thumb}
        posterResizeMode='contain'
      />

      {/* Loading State */}
      {isBuffering && (
        <View className='absolute inset-0 items-center justify-center bg-dark/50'>
          <Text className='text-light text-2xl'>Loading...</Text>
        </View>
      )}

      {/* Error State */}
      {error && (
        <View className='absolute inset-0 items-center justify-center bg-dark'>
          <Text className='text-danger text-2xl mb-4'>{error}</Text>
          <Pressable
            className='bg-secondary px-6 py-3 rounded-lg focus:scale-110'
            focusable={true}
            onPress={handleBack}
          >
            <Text className='text-dark text-xl font-bold'>Go Back</Text>
          </Pressable>
        </View>
      )}
    </View>
  );
}
