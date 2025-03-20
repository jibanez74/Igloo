import { Show, createEffect, createSignal } from "solid-js";
import { createMutation, createQuery } from "@tanstack/solid-query";
import ErrorWarning from "./ErrorWarning";
import { FiSave } from "solid-icons/fi";
import queryClient from "../utils/queryClient";
import type { Settings, SettingsResponse } from "../types/Settings";

export default function SettingsForm() {
  const [moviesDir, setMoviesDir] = createSignal<string>("");
  const [tvShowsDir, setTvShowsDir] = createSignal<string>("");
  const [musicDir, setMusicDir] = createSignal<string>("");
  const [moviesImgDir, setMoviesImgDir] = createSignal<string>("");
  const [studiosImgDir, setStudiosImgDir] = createSignal<string>("");
  const [artistsImgDir, setArtistsImgDir] = createSignal<string>("");
  const [avatarImgDir, setAvatarImgDir] = createSignal<string>("");
  const [staticDir, setStaticDir] = createSignal<string>("");
  const [downloadImages, setDownloadImages] = createSignal<boolean>(false);
  const [ffmpegPath, setFfmpegPath] = createSignal<string>("");
  const [ffprobePath, setFfprobePath] = createSignal<string>("");
  const [transcodeDir, setTranscodeDir] = createSignal<string>("");
  const [enableTranscoding, setEnableTranscoding] = createSignal(false);
  const [hardwareAcceleration, setHardwareAcceleration] = createSignal("");
  const [tmdbApiKey, setTmdbApiKey] = createSignal<string>("");
  const [jellyfinToken, setJellyfinToken] = createSignal<string>("");
  const [showSuccess, setShowSuccess] = createSignal(false);
  const [error, setError] = createSignal("");

  const query = createQuery(() => ({
    queryKey: ["settings"],
    queryFn: async (): Promise<SettingsResponse> => {
      try {
        const res = await fetch("/api/v1/settings", {
          credentials: "include",
        });

        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data;
      } catch (err) {
        console.error(err);
        throw new Error("Failed to fetch settings");
      }
    },
  }));

  const mutation = createMutation(() => ({
    mutationFn: async (input: Settings): Promise<SettingsResponse> => {
      const res = await fetch("/api/v1/settings", {
        method: "put",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(input),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(
          `${res.status} - ${data.error ? data.error : res.statusText}`
        );
      }

      return data;
    },
    onSuccess: () => {
      setShowSuccess(true);
      setTimeout(() => setShowSuccess(false), 3000);
      queryClient.invalidateQueries({ queryKey: ["settings"] });
    },
    onError: (error: Error) => {
      setError(error.message);
      setTimeout(() => setError(""), 5000);
      query.refetch();
    },
  }));

  const updateHandler = async (e: Event) => {
    e.preventDefault();
    setError(""); // Clear any previous errors

    const settings: Settings = {
      movies_dir_list: moviesDir(),
      tvshows_dir_list: tvShowsDir(),
      music_dir_list: musicDir(),
      movies_img_dir: moviesImgDir(),
      studios_img_dir: studiosImgDir(),
      artists_img_dir: artistsImgDir(),
      avatar_img_dir: avatarImgDir(),
      static_dir: staticDir(),
      download_images: downloadImages(),
      ffmpeg_path: ffmpegPath(),
      ffprobe_path: ffprobePath(),
      transcode_dir: transcodeDir(),
      enable_transcoding: enableTranscoding(),
      hardware_acceleration: hardwareAcceleration(),
      tmdb_api_key: tmdbApiKey(),
      jellyfin_token: jellyfinToken(),
    };

    mutation.mutate(settings);
  };

  createEffect(() => {
    if (query.isSuccess) {
      const { settings } = query.data;

      setMoviesDir(settings.movies_dir_list);
      setTvShowsDir(settings.tvshows_dir_list);
      setMusicDir(settings.music_dir_list);
      setMoviesImgDir(settings.movies_img_dir);
      setStudiosImgDir(settings.studios_img_dir);
      setArtistsImgDir(settings.artists_img_dir);
      setAvatarImgDir(settings.avatar_img_dir);
      setStaticDir(settings.static_dir);
      setDownloadImages(settings.download_images);
      setFfmpegPath(settings.ffmpeg_path);
      setFfprobePath(settings.ffprobe_path);
      setTranscodeDir(settings.transcode_dir);
      setEnableTranscoding(settings.enable_transcoding);
      setHardwareAcceleration(settings.hardware_acceleration);
      setTmdbApiKey(settings.tmdb_api_key);
      setJellyfinToken(settings.jellyfin_token);
    }
  });

  const isDisabled = () => query.isLoading || mutation.isPending;

  return (
    <article class="bg-blue-950/50 backdrop-blur-sm rounded-2xl p-6 shadow-lg shadow-blue-900/20">
      <Show when={error()}>
        <div class="mb-4">
          <ErrorWarning error={error()} isVisible={true} />
        </div>
      </Show>

      <Show when={showSuccess()}>
        <div class="mb-4 p-4 bg-green-500/10 border border-green-500/20 rounded-lg">
          <p class="text-green-400">Settings updated successfully!</p>
        </div>
      </Show>

      <form onSubmit={updateHandler} class="space-y-8">
        {/* Media Directories Section */}
        <section class="space-y-4">
          <header>
            <h2 class="text-xl font-semibold text-yellow-300">
              Media Directories
            </h2>
          </header>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label
                for="moviesDir"
                class="block text-sm font-medium text-blue-100"
              >
                Movies Directory
              </label>
              <input
                type="text"
                id="moviesDir"
                value={moviesDir()}
                onInput={(e) => setMoviesDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/movies"
              />
            </div>
            <div class="space-y-2">
              <label
                for="tvShowsDir"
                class="block text-sm font-medium text-blue-100"
              >
                TV Shows Directory
              </label>
              <input
                type="text"
                id="tvShowsDir"
                value={tvShowsDir()}
                onInput={(e) => setTvShowsDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/tvshows"
              />
            </div>
            <div class="space-y-2">
              <label
                for="musicDir"
                class="block text-sm font-medium text-blue-100"
              >
                Music Directory
              </label>
              <input
                type="text"
                id="musicDir"
                value={musicDir()}
                onInput={(e) => setMusicDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/music"
              />
            </div>
            <div class="space-y-2">
              <label
                for="staticDir"
                class="block text-sm font-medium text-blue-100"
              >
                Static Directory
              </label>
              <input
                type="text"
                id="staticDir"
                value={staticDir()}
                onInput={(e) => setStaticDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/static"
              />
            </div>
          </div>
        </section>

        {/* Image Directories Section */}
        <section class="space-y-4">
          <header>
            <h2 class="text-xl font-semibold text-yellow-300">
              Image Directories
            </h2>
          </header>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label
                for="moviesImgDir"
                class="block text-sm font-medium text-blue-100"
              >
                Movies Images
              </label>
              <input
                type="text"
                id="moviesImgDir"
                value={moviesImgDir()}
                onInput={(e) => setMoviesImgDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/movies/images"
              />
            </div>
            <div class="space-y-2">
              <label
                for="studiosImgDir"
                class="block text-sm font-medium text-blue-100"
              >
                Studios Images
              </label>
              <input
                type="text"
                id="studiosImgDir"
                value={studiosImgDir()}
                onInput={(e) => setStudiosImgDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/studios/images"
              />
            </div>
            <div class="space-y-2">
              <label
                for="artistsImgDir"
                class="block text-sm font-medium text-blue-100"
              >
                Artists Images
              </label>
              <input
                type="text"
                id="artistsImgDir"
                value={artistsImgDir()}
                onInput={(e) => setArtistsImgDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/artists/images"
              />
            </div>
            <div class="space-y-2">
              <label
                for="avatarImgDir"
                class="block text-sm font-medium text-blue-100"
              >
                Avatar Images
              </label>
              <input
                type="text"
                id="avatarImgDir"
                value={avatarImgDir()}
                onInput={(e) => setAvatarImgDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/avatars"
              />
            </div>
          </div>
        </section>

        {/* Transcoding Settings Section */}
        <section class="space-y-4">
          <header>
            <h2 class="text-xl font-semibold text-yellow-300">
              Transcoding Settings
            </h2>
          </header>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label
                for="ffmpegPath"
                class="block text-sm font-medium text-blue-100"
              >
                FFmpeg Path
              </label>
              <input
                type="text"
                id="ffmpegPath"
                value={ffmpegPath()}
                onInput={(e) => setFfmpegPath(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/usr/bin/ffmpeg"
              />
            </div>
            <div class="space-y-2">
              <label
                for="ffprobePath"
                class="block text-sm font-medium text-blue-100"
              >
                FFprobe Path
              </label>
              <input
                type="text"
                id="ffprobePath"
                value={ffprobePath()}
                onInput={(e) => setFfprobePath(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/usr/bin/ffprobe"
              />
            </div>
            <div class="space-y-2">
              <label
                for="transcodeDir"
                class="block text-sm font-medium text-blue-100"
              >
                Transcode Directory
              </label>
              <input
                type="text"
                id="transcodeDir"
                value={transcodeDir()}
                onInput={(e) => setTranscodeDir(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="/path/to/transcode"
              />
            </div>
            <div class="space-y-2">
              <label
                for="hardwareAcceleration"
                class="block text-sm font-medium text-blue-100"
              >
                Hardware Acceleration
              </label>
              <select
                id="hardwareAcceleration"
                value={hardwareAcceleration()}
                onInput={(e) => setHardwareAcceleration(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <option value="">None</option>
                <option value="h264_nvenc">NVIDIA NVENC (H.264)</option>
                <option value="h264_qsv">Intel QuickSync (H.264)</option>
                <option value="h264_amf">AMD AMF (H.264)</option>
              </select>
            </div>
          </div>
          <div class="flex items-center space-x-2">
            <input
              type="checkbox"
              id="enableTranscoding"
              checked={enableTranscoding()}
              onInput={(e) => setEnableTranscoding(e.currentTarget.checked)}
              disabled={isDisabled()}
              class="w-4 h-4 text-yellow-300 bg-blue-900/50 border-blue-700 rounded focus:ring-yellow-300/50 disabled:opacity-50 disabled:cursor-not-allowed"
            />
            <label
              for="enableTranscoding"
              class="text-sm font-medium text-blue-100"
            >
              Enable Transcoding
            </label>
          </div>
        </section>

        {/* API Settings Section */}
        <section class="space-y-4">
          <header>
            <h2 class="text-xl font-semibold text-yellow-300">API Settings</h2>
          </header>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label
                for="tmdbApiKey"
                class="block text-sm font-medium text-blue-100"
              >
                TMDB API Key
              </label>
              <input
                type="password"
                id="tmdbApiKey"
                value={tmdbApiKey()}
                onInput={(e) => setTmdbApiKey(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="Enter your TMDB API key"
              />
            </div>
            <div class="space-y-2">
              <label
                for="jellyfinToken"
                class="block text-sm font-medium text-blue-100"
              >
                Jellyfin Token
              </label>
              <input
                type="password"
                id="jellyfinToken"
                value={jellyfinToken()}
                onInput={(e) => setJellyfinToken(e.currentTarget.value)}
                disabled={isDisabled()}
                class="w-full px-3 py-2 bg-blue-900/50 border border-blue-700 rounded-lg text-blue-100 placeholder-blue-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
                placeholder="Enter your Jellyfin token"
              />
            </div>
          </div>
        </section>

        {/* Image Download Settings */}
        <section class="space-y-4">
          <header>
            <h2 class="text-xl font-semibold text-yellow-300">
              Image Settings
            </h2>
          </header>
          <div class="flex items-center space-x-2">
            <input
              type="checkbox"
              id="downloadImages"
              checked={downloadImages()}
              onInput={(e) => setDownloadImages(e.currentTarget.checked)}
              disabled={isDisabled()}
              class="w-4 h-4 text-yellow-300 bg-blue-900/50 border-blue-700 rounded focus:ring-yellow-300/50 disabled:opacity-50 disabled:cursor-not-allowed"
            />
            <label
              for="downloadImages"
              class="text-sm font-medium text-blue-100"
            >
              Download Images Automatically
            </label>
          </div>
        </section>

        {/* Submit Button */}
        <footer class="flex justify-end">
          <button
            type="submit"
            disabled={isDisabled()}
            class="inline-flex items-center px-4 py-2 bg-yellow-300 text-blue-900 font-medium rounded-lg hover:bg-yellow-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <FiSave class="w-5 h-5 mr-2" />
            {query.isLoading
              ? "Loading..."
              : mutation.isPending
                ? "Saving..."
                : "Save Settings"}
          </button>
        </footer>
      </form>
    </article>
  );
}
