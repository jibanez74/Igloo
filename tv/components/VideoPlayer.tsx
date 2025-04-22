import { useState, useRef, useEffect, useCallback } from "react";
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



export default function TvVideoPlayer({
  title,
  thumb,
  videoUri,
}: TvVideoPlayerProps) {
  const videoRef = useRef<VideoRef>(null);

  console.log("component rendering");

  return (
    <View style={styles.container}>
      <Video
        ref={videoRef}
        controls={Platform.OS === "ios"}
        fullscreen
        fullscreenAutorotate={false}
        fullscreenOrientation="landscape"
        enterPictureInPictureOnLeave={false}
        muted={false}
        onError={err => console.error(err)}
        playInBackground={false}
        poster={thumb}
        paused={false}
        renderToHardwareTextureAndroid={
          Platform.isTV && Platform.OS === "android"
        }
        resizeMode="contain"
        style={styles.video}
        source={{
          uri: videoUri,
          isNetwork: true,
          isLocalAssetFile: false,
          type: "mkv",
        }}
        volume={1}
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
