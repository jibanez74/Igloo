export type SettingsType = {
  music_dir: string | null;
  movies_dir: string | null;
  shows_dir: string | null;
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
};

export type GeneralSettingsResponseType = {
  settings: GeneralSettingsType;
};

export type UpdateGeneralSettingsResponseType = {
  settings: GeneralSettingsType;
  restart_required: boolean;
};
