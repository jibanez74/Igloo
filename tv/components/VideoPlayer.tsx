import { useState, useRef } from "react";
import { Platform, useTVEventHandler, View } from "react-native";
import Video, { OnBufferData, VideoRef } from "react-native-video";

type TvVideoPlayerProps = {
  title: string;
  thumb: string;
  videoUri: string;
};

const defaultMaxBitRate = 90000000;

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
  const videoRef = useRef<VideoRef>(null);
  const [isMuted, setIsMuted] = useState(false);
  const [isPaused, setIsPaused] = useState(true);
  const [isBuffering, setIsBuffering] = useState(true);
  const [currentTime, setCurrentTime] = useState(0);

  

  return (
    <View>
      <Video
        bufferConfig={defaultBufferConfig}
        controls
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
        ref={videoRef}
        renderToHardwareTextureAndroid={
          Platform.isTV && Platform.OS === "android"
        }
        source={{
          uri: videoUri,
          isNetwork: true,
        }}
      />
    </View>
  );
}
