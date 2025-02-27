export default function canPlayNativeHls(): boolean {
  const video = document.createElement('video');
  
  // Check if the browser can play HLS natively
  return video.canPlayType('application/vnd.apple.mpegurl') !== "";
}
