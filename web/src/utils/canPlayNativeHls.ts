export default function canPlayNativeHls() {
  const isSafari = /^((?!chrome|android).)*safari/i.test(navigator.userAgent);
  const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent);

  if (!isSafari && !isIOS) {
    return false;
  }

  const video = document.createElement("video");

  const hasHlsSupport =
    video.canPlayType("application/vnd.apple.mpegurl") !== "";
  const hasFmp4Support =
    video.canPlayType('video/mp4; codecs="mp4a.40.2,avc1.64001F"') !== "";

  return hasHlsSupport && hasFmp4Support;
}
