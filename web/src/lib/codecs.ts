import { getBrowser, compareVersions } from "./getBrowser";

type CodecSupport = {
  name: string;
  profiles?: string[];
  minVersion: {
    chrome?: string;
    firefox?: string;
    safari?: string;
    edge?: string;
  };
  containers: string[];
};

const videoCodecs: CodecSupport[] = [
  {
    name: "h264",
    profiles: ["baseline", "main", "high"],
    minVersion: {
      chrome: "58.0",
      firefox: "52.0",
      safari: "11.0",
      edge: "79.0",
    },
    containers: ["mp4", "webm"],
  },
  {
    name: "vp8",
    minVersion: {
      chrome: "25.0",
      firefox: "4.0",
      edge: "79.0",
    },
    containers: ["webm"],
  },
  {
    name: "vp9",
    minVersion: {
      chrome: "29.0",
      firefox: "28.0",
      edge: "79.0",
    },
    containers: ["webm"],
  },
  {
    name: "av1",
    minVersion: {
      chrome: "70.0",
      firefox: "67.0",
      edge: "79.0",
    },
    containers: ["mp4", "webm"],
  },
];

const audioCodecs: CodecSupport[] = [
  {
    name: "aac",
    minVersion: {
      chrome: "35.0",
      firefox: "52.0",
      safari: "11.0",
      edge: "79.0",
    },
    containers: ["mp4", "m4a"],
  },
  {
    name: "opus",
    minVersion: {
      chrome: "33.0",
      firefox: "15.0",
      edge: "79.0",
    },
    containers: ["webm"],
  },
  {
    name: "vorbis",
    minVersion: {
      chrome: "4.0",
      firefox: "3.6",
      edge: "79.0",
    },
    containers: ["webm", "ogg"],
  },
];

export function isVideoCodecSupported(
  codec: string,
  profile?: string
): boolean {
  const browser = getBrowser();
  const codecSupport = videoCodecs.find(c => c.name === codec.toLowerCase());

  if (!codecSupport) return false;

  // Check if browser is supported
  const minVersion = codecSupport.minVersion[browser.name];
  if (!minVersion) return false;

  // Check version compatibility
  if (compareVersions(browser.version, minVersion) < 0) return false;

  // Check profile if specified
  if (profile && codecSupport.profiles) {
    return codecSupport.profiles.includes(profile.toLowerCase());
  }

  return true;
}

export function isAudioCodecSupported(codec: string): boolean {
  const browser = getBrowser();
  const codecSupport = audioCodecs.find(c => c.name === codec.toLowerCase());

  if (!codecSupport) return false;

  // Check if browser is supported
  const minVersion = codecSupport.minVersion[browser.name];
  if (!minVersion) return false;

  // Check version compatibility
  return compareVersions(browser.version, minVersion) >= 0;
}

export function isMimeTypeSupported(mimeType: string): boolean {
  try {
    return MediaSource.isTypeSupported(mimeType);
  } catch (error) {
    console.error("Error checking MIME type support:", error);
    return false;
  }
}

export function getContainerSupport(
  codec: string,
  type: "video" | "audio"
): string[] {
  const codecs = type === "video" ? videoCodecs : audioCodecs;
  const codecSupport = codecs.find(c => c.name === codec.toLowerCase());
  return codecSupport?.containers || [];
}

// Example usage:
// const videoCodec = 'h264';
// const audioCodec = 'aac';
// const container = 'mp4';
//
// const isVideoSupported = isVideoCodecSupported(videoCodec, 'main');
// const isAudioSupported = isAudioCodecSupported(audioCodec);
// const mimeType = `video/mp4;codecs="avc1.42E01E,mp4a.40.2"`;
// const isMimeSupported = isMimeTypeSupported(mimeType);
//
// console.log(`Video codec supported: ${isVideoSupported}`);
// console.log(`Audio codec supported: ${isAudioSupported}`);
// console.log(`MIME type supported: ${isMimeSupported}`);
