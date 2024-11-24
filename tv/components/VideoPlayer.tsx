import { useState, useRef } from "react";
import {
  useTVEventHandler,
  View,
  Text,
  Pressable,
  TVEventHandler,
} from "react-native";
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

  useTVEventHandler(event => {
    switch (event.eventType) {
      case "playPause":
        setIsPaused(prev => !prev);
        setShowControls(true);
        break;

      case "select":
        setIsPaused(prev => !prev);
        setShowControls(true);
        break;

      case "right":
        videoRef.current?.seek(10);
        setShowControls(true);
        break;

      case "left":
        videoRef.current?.seek(-10);
        setShowControls(true);
        break;

      case "up":
        setShowControls(true);
        break;

      case "down":
        setShowControls(true);
        break;

      case "back":
      case "exit":
        handleBack();
        break;
    }

    // Hide controls after 3 seconds
    const timer = setTimeout(() => {
      setShowControls(false);
    }, 3000);

    return () => clearTimeout(timer);
  });

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

  // Configure ExoPlayer for direct streaming
  const videoConfig = {
    // Hardware acceleration
    androidHardwareAcceleration: true,
    useHardwareDecoder: true,

    // Buffer configuration for large files
    bufferConfig: {
      minBufferMs: 30000, // 30 seconds minimum buffer
      maxBufferMs: 120000, // 2 minutes maximum buffer
      bufferForPlaybackMs: 5000, // Start playback after 5 seconds
      bufferForPlaybackAfterRebufferMs: 10000, // 10 seconds after rebuffer
    },

    maxBitRate:
      resolution >= 2160
        ? 90000000 // 80Mbps for 4K
        : resolution >= 1080
        ? 40000000 // 40Mbps for 1080p
        : 20000000, // 20Mbps for others

    // Playback settings
    progressUpdateInterval: 250,
    reportBandwidth: true,

    // Cache settings for large files
    cache: true,
    maxCacheSize: 1024 * 1024 * 1024, // 1GB cache

    // Prevent sleep during playback
    preventsDisplaySleepDuringVideoPlayback: true,
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
        className='absolute inset-0 w-full h-full'
        resizeMode='contain'
        paused={isPaused}
        onBuffer={handleBuffer}
        onError={handleError}
        repeat={false}
        controls={false}
        fullscreen={true}
        poster={thumb}
        posterResizeMode='contain'
        {...videoConfig}
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

      {/* Controls Overlay */}
      {showControls && (
        <View className='absolute inset-0 items-center justify-between p-8'>
          {/* Top Bar */}
          <View className='w-full flex-row items-center'>
            <Pressable
              className='flex-row items-center bg-dark/80 px-4 py-2 rounded-lg
                        focus:bg-secondary/20 focus:scale-110'
              focusable={true}
              onPress={handleBack}
            >
              <Ionicons name='arrow-back' size={24} color='#CEE3F9' />
              <Text className='text-light text-xl ml-2'>Back</Text>
            </Pressable>
          </View>

          {/* Center Controls */}
          <Pressable
            className='bg-dark/80 p-4 rounded-full
                      focus:bg-secondary/20 focus:scale-110'
            focusable={true}
            onPress={() => setIsPaused(!isPaused)}
          >
            <Ionicons
              name={isPaused ? "play" : "pause"}
              size={48}
              color='#CEE3F9'
            />
          </Pressable>

          {/* Progress Bar */}
          <View className='w-full h-1 bg-dark/80 rounded-full'>
            <View className='h-full w-1/2 bg-secondary rounded-full' />
          </View>
        </View>
      )}
    </View>
  );
}
