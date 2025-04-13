export type Settings = {
  movies_dir_list: string;
  movies_img_dir: string;
  music_dir_list: string;
  tvshows_dir_list: string;
  transcode_dir: string;
  studios_img_dir: string;
  static_dir: string;
  artists_img_dir: string;
  avatar_img_dir: string;
  download_images: boolean;
  tmdb_api_key: string;
  ffmpeg_path: string;
  ffprobe_path: string;
  hardware_acceleration: string;
  enable_transcoding: boolean;
  jellyfin_token: string;
};

export type SettingsResponse = {
  settings: Settings;
};
