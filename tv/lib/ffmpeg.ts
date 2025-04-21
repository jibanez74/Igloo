import { FFmpegKit, ReturnCode } from 'ffmpeg-kit-react-native';

export async function initializeFFmpeg() {
  try {
    // Check if FFmpeg is already loaded
    const session = await FFmpegKit.execute('-version');
    const returnCode = await session.getReturnCode();

    if (ReturnCode.isSuccess(returnCode)) {
      console.log('FFmpeg initialized successfully');
      return true;
    } else {
      console.error('FFmpeg initialization failed');
      return false;
    }
  } catch (error) {
    console.error('Error initializing FFmpeg:', error);
    return false;
  }
}

export async function getFFmpegVersion() {
  try {
    const session = await FFmpegKit.execute('-version');
    const output = await session.getOutput();
    return output;
  } catch (error) {
    console.error('Error getting FFmpeg version:', error);
    return null;
  }
}

// Example function to convert video format
export async function convertVideo(inputPath: string, outputPath: string) {
  try {
    const session = await FFmpegKit.execute(`-i ${inputPath} -c:v libx264 -c:a aac ${outputPath}`);
    const returnCode = await session.getReturnCode();

    if (ReturnCode.isSuccess(returnCode)) {
      console.log('Video conversion successful');
      return true;
    } else {
      console.error('Video conversion failed');
      return false;
    }
  } catch (error) {
    console.error('Error converting video:', error);
    return false;
  }
} 