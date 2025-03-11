export type MovieHlsOpts = {
  title: string;
  audio_codec: string;
  audio_stream_index?: number;
  audio_bit_rate?: number;
  audio_channels?: number;
  video_codec: string;
  video_stream_index?: number;
  video_bit_rate?: number;
  video_height?: number;
  video_profile?: string;
  preset?: string;
};
