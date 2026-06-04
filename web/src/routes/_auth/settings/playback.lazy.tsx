import { createLazyFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState, useTransition } from "react";
import type { FormEvent } from "react";
import {
  Gauge,
  Languages,
  Play,
  RotateCcw,
  Save,
  Sliders,
  Subtitles,
  Wifi,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import {
  GENERAL_SETTINGS_KEY,
  LANGUAGE_NAMES,
  PLAYBACK_SETTINGS_KEY,
} from "@/lib/constants";
import { updatePlaybackSettings } from "@/lib/api";
import { recommendedProfileId } from "@/lib/playback-recommendation";
import { authUserQueryOpts, playbackSettingsQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showSuccess,
  showValidationError,
} from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  PlaybackSettingsType,
  UpdatePlaybackSettingsRequest,
} from "@/types";

export const Route = createLazyFileRoute("/_auth/settings/playback")({
  component: PlaybackSettings,
});

const NO_PROFILE_VALUE = "__none__";
const NO_LANGUAGE_VALUE = "__none__";
const SUBTITLE_OFF_VALUE = "off";
const LANGUAGE_CODE_PATTERN = /^[a-z]{2,3}$/;
const DOWNLOAD_SPEED_VALIDATION_MESSAGE =
  "Download speed must be between 0 and 10000 Mbps.";
const SERVER_UPLOAD_VALIDATION_MESSAGE =
  "Server upload bandwidth must be greater than 0 and less than 100000 Mbps.";

const SORTED_LANGUAGE_ENTRIES = Object.entries(LANGUAGE_NAMES).sort(
  ([, a], [, b]) => a.localeCompare(b),
);

function formFromSettings(
  settings: PlaybackSettingsType,
): UpdatePlaybackSettingsRequest {
  const form: UpdatePlaybackSettingsRequest = {
    preferred_profile: settings.preferred_profile,
    download_mbps: settings.download_mbps,
    preferred_audio_language: settings.preferred_audio_language,
    preferred_subtitle_language: settings.preferred_subtitle_language,
  };

  if (settings.is_admin) {
    form.server_upload_mbps = settings.server_upload_mbps;
  }

  return form;
}

function PlaybackSettings() {
  const { data: authData, isLoading: authLoading } = useQuery(
    authUserQueryOpts(),
  );
  const userId =
    authData?.error === false && authData.data?.user
      ? authData.data.user.id
      : null;
  const { data, isLoading } = useQuery({
    ...playbackSettingsQueryOpts(userId ?? 0),
    enabled: userId !== null,
  });

  const settings =
    data?.error === false && data.data?.settings ? data.data.settings : null;

  if (authLoading || isLoading) {
    return <PlaybackSettingsLoading />;
  }

  if (data?.error) {
    return (
      <div className="max-w-3xl animate-in duration-300 fade-in">
        <Card className="border-red-500/20 bg-red-500/10">
          <CardHeader>
            <CardTitle className="text-red-300">
              Settings unavailable
            </CardTitle>
            <CardDescription className="text-red-200/80">
              {data.message || "Failed to load playback settings."}
            </CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  if (!settings) {
    return null;
  }

  return <PlaybackSettingsForm settings={settings} />;
}

type PlaybackSettingsFormProps = {
  settings: PlaybackSettingsType;
};

function PlaybackSettingsForm({ settings }: PlaybackSettingsFormProps) {
  const queryClient = useQueryClient();
  const downloadMbpsId = useId();
  const preferredProfileId = useId();
  const recommendationId = useId();
  const recommendationTitleId = useId();
  const serverUploadMbpsId = useId();
  const statusId = useId();
  const preferredAudioLanguageId = useId();
  const preferredSubtitleLanguageId = useId();

  const [form, setForm] = useState<UpdatePlaybackSettingsRequest>(() =>
    formFromSettings(settings),
  );
  const [syncedSettings, setSyncedSettings] = useState(settings);
  const [validationMessage, setValidationMessage] = useState("");
  const [, startTransition] = useTransition();

  if (settings !== syncedSettings) {
    setSyncedSettings(settings);
    setForm(formFromSettings(settings));
    setValidationMessage("");
  }

  const updateMutation = useMutation({
    mutationFn: updatePlaybackSettings,
    onSuccess: res => {
      if (res.error) {
        showActionFailed("save playback settings", res.message);
        return;
      }
      queryClient.invalidateQueries({ queryKey: [PLAYBACK_SETTINGS_KEY] });
      queryClient.invalidateQueries({ queryKey: [GENERAL_SETTINGS_KEY] });
      showSuccess("Playback settings saved");
    },
    onError: () => {
      showActionFailed(
        "save playback settings",
        "An unexpected error occurred",
      );
    },
  });

  const handleDownloadMbpsChange = (value: string) => {
    const trimmed = value.trim();
    if (trimmed === "") {
      setForm(current => ({ ...current, download_mbps: null }));
      return;
    }
    const parsed = Number.parseFloat(trimmed);
    setForm(current => ({
      ...current,
      download_mbps: Number.isFinite(parsed) ? parsed : null,
    }));
  };

  const handleServerUploadMbpsChange = (value: string) => {
    const trimmed = value.trim();
    if (trimmed === "") {
      setForm(current => ({ ...current, server_upload_mbps: null }));
      return;
    }
    const parsed = Number.parseFloat(trimmed);
    setForm(current => ({
      ...current,
      server_upload_mbps: Number.isFinite(parsed) ? parsed : null,
    }));
  };

  const handleProfileChange = (value: string) => {
    startTransition(() => {
      setForm(current => ({
        ...current,
        preferred_profile: value === NO_PROFILE_VALUE ? null : value,
      }));
    });
  };

  const handleAudioLanguageChange = (value: string) => {
    startTransition(() => {
      setForm(current => ({
        ...current,
        preferred_audio_language:
          value === NO_LANGUAGE_VALUE ? null : value,
      }));
    });
  };

  const handleSubtitleLanguageChange = (value: string) => {
    startTransition(() => {
      setForm(current => ({
        ...current,
        preferred_subtitle_language:
          value === NO_LANGUAGE_VALUE ? null : value,
      }));
    });
  };

  const resetForm = () => {
    setForm(formFromSettings(settings));
    setValidationMessage("");
  };

  const validateForm = () => {
    if (
      form.download_mbps != null &&
      (form.download_mbps <= 0 || form.download_mbps >= 10000)
    ) {
      return DOWNLOAD_SPEED_VALIDATION_MESSAGE;
    }
    if (
      form.preferred_profile != null &&
      !settings.profiles.some(p => p.id === form.preferred_profile)
    ) {
      return "Selected profile is not available.";
    }
    if (
      form.preferred_audio_language != null &&
      !LANGUAGE_CODE_PATTERN.test(form.preferred_audio_language)
    ) {
      return "Audio language must be a 2- or 3-letter lowercase code.";
    }
    if (
      form.preferred_subtitle_language != null &&
      form.preferred_subtitle_language !== SUBTITLE_OFF_VALUE &&
      !LANGUAGE_CODE_PATTERN.test(form.preferred_subtitle_language)
    ) {
      return "Subtitle language must be a 2- or 3-letter lowercase code.";
    }
    if (
      settings.is_admin &&
      form.server_upload_mbps != null &&
      (form.server_upload_mbps <= 0 || form.server_upload_mbps >= 100000)
    ) {
      return SERVER_UPLOAD_VALIDATION_MESSAGE;
    }
    return "";
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const message = validateForm();
    if (message) {
      setValidationMessage(message);
      showValidationError(message);
      return;
    }
    setValidationMessage("");
    updateMutation.mutate(form);
  };

  const recommendedId = recommendedProfileId(
    settings.profiles,
    form.download_mbps,
    settings.is_admin
      ? (form.server_upload_mbps ?? null)
      : settings.server_upload_mbps,
  );
  const recommendedProfile = recommendedId
    ? settings.profiles.find(p => p.id === recommendedId)
    : null;
  const downloadMbpsInvalid =
    validationMessage === DOWNLOAD_SPEED_VALIDATION_MESSAGE &&
    form.download_mbps != null &&
    (form.download_mbps <= 0 || form.download_mbps >= 10000);
  const serverUploadMbpsInvalid =
    validationMessage === SERVER_UPLOAD_VALIDATION_MESSAGE &&
    settings.is_admin &&
    form.server_upload_mbps != null &&
    (form.server_upload_mbps <= 0 || form.server_upload_mbps >= 100000);

  return (
    <form
      onSubmit={handleSubmit}
      noValidate
      className="max-w-5xl animate-in space-y-6 duration-300 fade-in"
    >
      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle
            role="heading"
            aria-level={2}
            className="flex items-center gap-2 text-white"
          >
            <Play className="size-5 text-amber-400" aria-hidden="true" />
            Playback Settings
          </CardTitle>
          <CardDescription className="text-slate-300">
            Tell Igloo about your connection so it can pick the right stream
            quality for you.
          </CardDescription>
        </CardHeader>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Gauge className="size-5 text-amber-400" aria-hidden="true" />
            Your network speed
          </CardTitle>
          <CardDescription className="text-slate-300">
            Used to recommend a stream profile for your viewing.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid max-w-md gap-2">
            <Label htmlFor={downloadMbpsId}>Download speed (Mbps)</Label>
            <Input
              id={downloadMbpsId}
              name="download_mbps"
              type="number"
              inputMode="decimal"
              min={0.1}
              step={0.1}
              value={form.download_mbps ?? ""}
              onChange={event => handleDownloadMbpsChange(event.target.value)}
              disabled={updateMutation.isPending}
              aria-invalid={downloadMbpsInvalid ? "true" : undefined}
              aria-describedby={
                downloadMbpsInvalid
                  ? `${downloadMbpsId}-description ${statusId}`
                  : `${downloadMbpsId}-description`
              }
              className="h-10 border-slate-600 bg-slate-950/60 text-white placeholder:text-slate-500 focus-visible:ring-amber-400/30"
            />
            <p
              id={`${downloadMbpsId}-description`}
              className="text-sm text-slate-400"
            >
              Leave blank if unsure.
            </p>
          </div>
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Wifi className="size-5 text-amber-400" aria-hidden="true" />
            Server upload cap
          </CardTitle>
          <CardDescription className="text-slate-300">
            The home server&apos;s outbound bandwidth limit, applied when you
            stream from outside the home network.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {settings.is_admin ? (
            <div className="grid max-w-md gap-2">
              <Label htmlFor={serverUploadMbpsId}>
                Server upload bandwidth (Mbps)
              </Label>
              <Input
                id={serverUploadMbpsId}
                name="server_upload_mbps"
                type="number"
                inputMode="decimal"
                min={0.1}
                step={0.1}
                value={form.server_upload_mbps ?? ""}
                onChange={event =>
                  handleServerUploadMbpsChange(event.target.value)
                }
                disabled={updateMutation.isPending}
                aria-invalid={serverUploadMbpsInvalid ? "true" : undefined}
                aria-describedby={
                  serverUploadMbpsInvalid
                    ? `${serverUploadMbpsId}-description ${statusId}`
                    : `${serverUploadMbpsId}-description`
                }
                className="h-10 border-slate-600 bg-slate-950/60 text-white placeholder:text-slate-500 focus-visible:ring-amber-400/30"
              />
              <p
                id={`${serverUploadMbpsId}-description`}
                className="text-sm text-slate-400"
              >
                Leave blank if the server should be uncapped.
              </p>
            </div>
          ) : (
            <>
              <p className="text-sm text-white">
                {settings.server_upload_mbps != null
                  ? `${settings.server_upload_mbps} Mbps`
                  : "Not set (uncapped)"}
              </p>
              <p className="mt-2 text-sm text-slate-400">
                Set by the server administrator. Affects your recommendation
                when streaming from outside the home network.
              </p>
            </>
          )}
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle
            id={recommendationTitleId}
            className="flex items-center gap-2 text-white"
          >
            <Sliders className="size-5 text-amber-400" aria-hidden="true" />
            Recommended profile
          </CardTitle>
          <CardDescription className="text-slate-300">
            Calculated from your download speed and the server upload cap, with
            20% headroom for audio and overhead.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p
            id={recommendationId}
            role="region"
            aria-labelledby={recommendationTitleId}
            aria-live="polite"
            className="text-sm text-white"
          >
            {recommendedProfile
              ? `Recommended: ${recommendedProfile.label}`
              : "Enter your download speed to see a recommendation."}
          </p>
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Play className="size-5 text-amber-400" aria-hidden="true" />
            Preferred profile
          </CardTitle>
          <CardDescription className="text-slate-300">
            The default profile when you start a stream. You can still pick a
            different one per video.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid max-w-md gap-2">
            <Label htmlFor={preferredProfileId}>Profile</Label>
            <Select
              name="preferred_profile"
              value={form.preferred_profile ?? NO_PROFILE_VALUE}
              onValueChange={handleProfileChange}
              disabled={updateMutation.isPending}
            >
              <SelectTrigger
                id={preferredProfileId}
                className="h-10 w-full border-slate-600 bg-slate-950/60 text-white shadow-none focus-visible:ring-amber-400/30"
              >
                <SelectValue placeholder="Use recommended" />
              </SelectTrigger>
              <SelectContent className="border-slate-700 bg-slate-900 text-slate-100">
                <SelectItem
                  value={NO_PROFILE_VALUE}
                  className="focus:bg-slate-800 focus:text-white"
                >
                  Use recommended
                </SelectItem>
                {settings.profiles.map(profile => (
                  <SelectItem
                    key={profile.id}
                    value={profile.id}
                    className="focus:bg-slate-800 focus:text-white"
                  >
                    {profile.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Languages className="size-5 text-amber-400" aria-hidden="true" />
            Preferred audio language
          </CardTitle>
          <CardDescription className="text-slate-300">
            Igloo will pick the matching audio track when a movie has one. You
            can still change it per video.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid max-w-md gap-2">
            <Label htmlFor={preferredAudioLanguageId}>Audio language</Label>
            <Select
              name="preferred_audio_language"
              value={form.preferred_audio_language ?? NO_LANGUAGE_VALUE}
              onValueChange={handleAudioLanguageChange}
              disabled={updateMutation.isPending}
            >
              <SelectTrigger
                id={preferredAudioLanguageId}
                className="h-10 w-full border-slate-600 bg-slate-950/60 text-white shadow-none focus-visible:ring-amber-400/30"
              >
                <SelectValue placeholder="No preference" />
              </SelectTrigger>
              <SelectContent className="border-slate-700 bg-slate-900 text-slate-100">
                <SelectItem
                  value={NO_LANGUAGE_VALUE}
                  className="focus:bg-slate-800 focus:text-white"
                >
                  No preference
                </SelectItem>
                {SORTED_LANGUAGE_ENTRIES.map(([code, name]) => (
                  <SelectItem
                    key={code}
                    value={code}
                    className="focus:bg-slate-800 focus:text-white"
                  >
                    {name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      <Card className="border-slate-700/50 bg-slate-800/30 transition-colors duration-200">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Subtitles className="size-5 text-amber-400" aria-hidden="true" />
            Preferred subtitles
          </CardTitle>
          <CardDescription className="text-slate-300">
            Pick a default subtitle language, or always start with subtitles
            off.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid max-w-md gap-2">
            <Label htmlFor={preferredSubtitleLanguageId}>
              Subtitle language
            </Label>
            <Select
              name="preferred_subtitle_language"
              value={form.preferred_subtitle_language ?? NO_LANGUAGE_VALUE}
              onValueChange={handleSubtitleLanguageChange}
              disabled={updateMutation.isPending}
            >
              <SelectTrigger
                id={preferredSubtitleLanguageId}
                className="h-10 w-full border-slate-600 bg-slate-950/60 text-white shadow-none focus-visible:ring-amber-400/30"
              >
                <SelectValue placeholder="No preference" />
              </SelectTrigger>
              <SelectContent className="border-slate-700 bg-slate-900 text-slate-100">
                <SelectItem
                  value={NO_LANGUAGE_VALUE}
                  className="focus:bg-slate-800 focus:text-white"
                >
                  No preference
                </SelectItem>
                <SelectItem
                  value={SUBTITLE_OFF_VALUE}
                  className="focus:bg-slate-800 focus:text-white"
                >
                  Always off
                </SelectItem>
                {SORTED_LANGUAGE_ENTRIES.map(([code, name]) => (
                  <SelectItem
                    key={code}
                    value={code}
                    className="focus:bg-slate-800 focus:text-white"
                  >
                    {name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      <div className="rounded-lg border border-slate-700/50 bg-slate-900/70 p-4 shadow-lg shadow-black/10 transition-colors duration-200 sm:flex sm:items-center sm:justify-between sm:gap-4">
        <div className="min-w-0">
          <p className="text-sm font-medium text-white">Playback settings</p>
          <p
            id={statusId}
            className={cn(
              "mt-1 text-sm transition-colors",
              validationMessage ? "text-red-300" : "text-slate-400",
            )}
            aria-live="polite"
          >
            {validationMessage ||
              "Saved preferences apply to your future streams."}
          </p>
        </div>
        <div className="mt-4 flex flex-col gap-2 sm:mt-0 sm:flex-row">
          <Button
            type="button"
            variant="outline"
            onClick={resetForm}
            disabled={updateMutation.isPending}
            className="border-slate-600 bg-slate-800/90 text-slate-100 hover:bg-slate-700 hover:text-white"
          >
            <RotateCcw className="size-4" aria-hidden="true" />
            Reset
          </Button>
          <Button
            type="submit"
            variant="accent"
            disabled={updateMutation.isPending}
          >
            {updateMutation.isPending ? (
              <Spinner className="size-4" aria-hidden="true" />
            ) : (
              <Save className="size-4" aria-hidden="true" />
            )}
            {updateMutation.isPending ? "Saving..." : "Save Settings"}
          </Button>
        </div>
      </div>
    </form>
  );
}

function PlaybackSettingsLoading() {
  const loadingId = useId();

  return (
    <div
      className="max-w-5xl animate-in space-y-6 duration-300 fade-in"
      role="status"
      aria-labelledby={loadingId}
    >
      <Card className="border-slate-700/50 bg-slate-800/30">
        <CardContent className="flex min-h-40 items-center justify-center">
          <div className="flex items-center gap-3 text-slate-300">
            <Spinner className="size-5 text-amber-400" aria-hidden="true" />
            <span id={loadingId}>Loading playback settings...</span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
