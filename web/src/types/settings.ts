export type SettingsType = {
  music_dir: string | null;
  movies_dir: string | null;
  shows_dir: string | null;
};

export type UpdateLibrarySettingsRequest = SettingsType;

export type UpdateLibrarySettingsResponseType = {
  settings: SettingsType;
};

export type HardwareAccelerationDevice = "cpu" | "apple" | "nvidia" | "intel";

export type GeneralSettingsType = {
  tmdb_key: string | null;
  jellyfin_token: string | null;
  spotify_client_id: string | null;
  spotify_client_secret: string | null;
  hardware_acceleration_device: HardwareAccelerationDevice;
  enable_logger: boolean;
  enable_watcher: boolean;
  download_images: boolean;
  static_dir: string;
  logs_dir: string;
  transcode_dir: string;
  server_upload_mbps: number | null;
  restart_required?: boolean;
};

export type UpdateGeneralSettingsRequest = {
  tmdb_key: string;
  jellyfin_token: string;
  spotify_client_id: string;
  spotify_client_secret: string;
  hardware_acceleration_device: HardwareAccelerationDevice;
  enable_logger: boolean;
  enable_watcher: boolean;
  download_images: boolean;
  static_dir: string;
  logs_dir: string;
  transcode_dir: string;
  server_upload_mbps: number | null;
};

export type GeneralSettingsResponseType = {
  settings: GeneralSettingsType;
};

export type UpdateGeneralSettingsResponseType = {
  settings: GeneralSettingsType;
  restart_required: boolean;
};

export type PlaybackProfileType = {
  id: string;
  label: string;
  height: number;
  video_mbps: number;
};

export type PlaybackSettingsType = {
  profiles: PlaybackProfileType[];
  preferred_profile: string | null;
  download_mbps: number | null;
  server_upload_mbps: number | null;
  is_admin: boolean;
  preferred_audio_language: string | null;
  preferred_subtitle_language: string | null;
};

export type UpdatePlaybackSettingsRequest = {
  preferred_profile: string | null;
  download_mbps: number | null;
  preferred_audio_language: string | null;
  preferred_subtitle_language: string | null;
};

export type PlaybackSettingsResponseType = {
  settings: PlaybackSettingsType;
};

export type UpdatePlaybackSettingsResponseType = {
  settings: {
    preferred_profile: string | null;
    download_mbps: number | null;
    preferred_audio_language: string | null;
    preferred_subtitle_language: string | null;
  };
};
