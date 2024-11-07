import { StyleSheet, View, Dimensions } from 'react-native';
import Video from 'react-native-video';
import { useState, useRef } from 'react';
import Colors from '@/constants/Colors';

interface ExoVideoPlayerProps {
  uri: string;
  audioTracks?: Array<{ language: string; title: string }>;
  subtitleTracks?: Array<{ language: string; title: string }>;
}

const { width: SCREEN_WIDTH, height: SCREEN_HEIGHT } = Dimensions.get('window');

export default function ExoVideoPlayer({ uri, audioTracks, subtitleTracks }: ExoVideoPlayerProps) {
  const videoRef = useRef<Video>(null);
  const [paused, setPaused] = useState(false);

  return (
    <View style={styles.container}>
      <Video
        ref={videoRef}
        source={{ uri }}
        style={styles.video}
        resizeMode="contain"
        paused={paused}
        controls={true}
        fullscreen={true}
        repeat={false}
        onLoad={(data) => {
          console.log('Video loaded:', data);
        }}
        onError={(error) => {
          console.error('Video error:', error);
        }}
        // ExoPlayer specific props
        selectedAudioTrack={{
          type: 'language',
          value: audioTracks?.[0]?.language || 'en',
        }}
        selectedTextTrack={{
          type: 'language',
          value: subtitleTracks?.[0]?.language || 'en',
        }}
        // TV specific props
        focusable={true}
        nextFocusDown={undefined}
        nextFocusUp={undefined}
        nextFocusLeft={undefined}
        nextFocusRight={undefined}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: Colors.primary,
    justifyContent: 'center',
    alignItems: 'center',
  },
  video: {
    width: SCREEN_WIDTH,
    height: SCREEN_HEIGHT,
  },
}); 