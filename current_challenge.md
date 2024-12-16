# Media Streaming Platform Overview

## Platform Overview
You are building a media streaming platform similar to Jellyfin or Plex with:
- **Backend**: Written in Go.
- **Web Client**: Built with React.
- **TV App**: Developed in React Native.
- **Focus**: Currently working on the web client.

---

## Goal
To enable smooth movie playback in the web browser using **ReactPlayer** while ensuring compatibility with the browser and optimizing performance.

---

## Challenges
1. **Verifying if a video file is playable in the browser**:
   - Considering format (e.g., MP4, MKV, WebM).
   - Considering video codecs (e.g., H.264, H.265).
   - Considering audio codecs (e.g., AAC, AC3, DTS, TrueHD).

2. **Ensuring videos are seekable** for users (jumping to any part of the video).

---

## Plan of Attack
1. **Always Serve HLS Streams**:
   - HLS will be used to ensure compatibility with browsers and ReactPlayer.
   - HLS supports adaptive streaming and smooth seeking.

2. **FFmpeg for HLS Generation**:
   - FFmpeg will be used in the Go backend to generate HLS streams.

3. **Codec Verification**:
   - The backend will check the video and audio codecs for browser compatibility.
   - If codecs are compatible:
     - Use FFmpeg's `-c:v copy` and `-c:a copy` flags to avoid transcoding and directly copy the streams into the HLS format.
   - If codecs are incompatible:
     - Transcode the video to H.264 and the audio to AAC, which are widely supported by browsers.

4. **Seekable Video**:
   - HLS playlists and segments will be configured with appropriate keyframes and segment durations to ensure smooth seeking.

5. **File Storage**:
   - The backend will write the generated HLS segments and playlists to disk.
   - These files will be served directly to the client for playback.

---

## Decision: Writing HLS Files to Disk

### **Reasoning**:
- **Reusability**:
  - Pre-generated HLS files can be used by multiple clients without reprocessing.

- **Scalability**:
  - HLS files can be served via a file server or CDN, reducing backend load.

- **Smooth Seeking**:
  - Pre-generated playlists and segments improve seek performance.

- **Efficiency**:
  - CPU usage is minimized for subsequent playback of the same file.

---

## Using Fiber for HLS Streaming
### **Advantages of Fiber**:
1. **High Throughput for Serving HLS Segments**:
   - Fiber is built on `fasthttp`, providing faster response times and better handling of concurrent requests for HLS segments.

2. **Efficient Static File Serving**:
   - Fiber includes an optimized static file server for serving HLS `.m3u8` playlists and `.ts` segments.

3. **Reduced Latency**:
   - Fiber’s lightweight framework and asynchronous handling reduce latency in serving requests.

4. **Optimized for High Concurrency**:
   - Fiber can handle high-concurrency workloads more efficiently, making it ideal for streaming to multiple clients.

5. **Built-in Middleware**:
   - Features like rate limiting and caching help manage resources and improve repeated request performance.

### **When Fiber Excels**:
- Real-time streaming scenarios where performance is critical.
- High-traffic platforms that require scalability and low latency.
- Serving HLS files dynamically with minimal resource overhead.

---

## Utility Functions for Codec Compatibility

### Video Codec Compatibility Check
```typescript
// Define a list of browser-compatible video codecs
const compatibleVideoCodecs: string[] = [
  "h264", // H.264 (MPEG-4 AVC)
  "vp8",  // VP8
  "vp9",  // VP9
  "av1",  // AV1
];

// Function to check if a video codec is browser-compatible
function isVideoCodecCompatible(codec: string): boolean {
  // Normalize codec string to lowercase for comparison
  const normalizedCodec = codec.toLowerCase();

  // Check if the codec exists in the list of compatible codecs
  return compatibleVideoCodecs.includes(normalizedCodec);
}

// Example usage:
console.log(isVideoCodecCompatible("h264")); // true
console.log(isVideoCodecCompatible("h265")); // false
console.log(isVideoCodecCompatible("vp9"));  // true
```

### Audio Codec Compatibility Check
```typescript
// Define a list of browser-compatible audio codecs
const compatibleAudioCodecs: string[] = [
  "aac",  // Advanced Audio Codec
  "mp3",  // MPEG Layer-3 Audio
  "opus", // Opus
  "vorbis", // Vorbis (used in WebM containers)
];

// Function to check if an audio codec is browser-compatible
function isAudioCodecCompatible(codec: string): boolean {
  // Normalize codec string to lowercase for comparison
  const normalizedCodec = codec.toLowerCase();

  // Check if the codec exists in the list of compatible codecs
  return compatibleAudioCodecs.includes(normalizedCodec);
}

// Example usage:
console.log(isAudioCodecCompatible("aac"));    // true
console.log(isAudioCodecCompatible("ac3"));    // false
console.log(isAudioCodecCompatible("opus"));   // true
```

---

## Next Steps
1. Implement FFmpeg commands in the Go backend to:
   - Generate HLS segments and playlists.
   - Verify and handle codec compatibility.

2. Build logic for serving HLS files to clients using Fiber for optimal performance.

3. Ensure cleanup of unused HLS files to manage storage effectively.

4. Test playback and seek functionality across different browsers to verify compatibility and performance.

5. Evaluate the integration of Fiber for HLS file serving to leverage its performance advantages.
