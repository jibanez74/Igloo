import { useId, useState } from "react";
import { Gauge, Sliders } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import PlaybackSection from "@/components/settings/PlaybackSection";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import {
  DOWNLOAD_SPEED_MAX_MBPS,
  LANGUAGE_NAMES,
  MOTION_SETTINGS_SURFACE_CLASS,
  SETTINGS_CARD_SURFACE_CLASS,
  SETTINGS_INPUT_CLASS,
  SETTINGS_SELECT_CONTENT_CLASS,
  SETTINGS_SELECT_ITEM_CLASS,
  SETTINGS_SELECT_TRIGGER_CLASS,
  SUBTITLE_OFF_VALUE,
} from "@/lib/constants";
import { useDevicePlaybackPreferences } from "@/hooks/useDevicePlaybackPreferences";
import { parseMbpsInput } from "@/lib/playback";
import { setDevicePlaybackPreferences } from "@/lib/playback-preferences";
import { recommendedProfileId } from "@/lib/playback-recommendation";
import { cn } from "@/lib/utils";
import type { DevicePlaybackPreferences, PlaybackSettingsType } from "@/types";

const NO_SELECTION_VALUE = "__none__";
const DOWNLOAD_SPEED_VALIDATION_MESSAGE =
  `Download speed must be between 0 and ${DOWNLOAD_SPEED_MAX_MBPS} Mbps.`;
/** Explains that the device card writes to this browser rather than the account. */
const DEVICE_SCOPE_DESCRIPTION =
  "Saved in this browser only. Your other devices keep their own settings.";
const DEVICE_SAVED_SUFFIX = "Saved on this device.";
/** Used instead of DEVICE_SAVED_SUFFIX when localStorage refused the write. */
const DEVICE_SESSION_ONLY_SUFFIX =
  "Applied for this session only; this browser is not saving settings.";
const DEVICE_PERSISTENCE_FAILED_MESSAGE =
  "This browser is not saving settings, so these choices are lost when the page reloads. Private browsing or a full storage quota is the usual cause.";

const LANGUAGE_OPTIONS = Object.entries(LANGUAGE_NAMES)
  .sort(([, a], [, b]) => a.localeCompare(b))
  .map(([value, label]) => ({ value, label }));

// Subtitles add an explicit "off" on top of the languages; audio cannot be
// turned off, so it uses the plain list.
const SUBTITLE_OPTIONS = [
  { value: SUBTITLE_OFF_VALUE, label: "Always off" },
  ...LANGUAGE_OPTIONS,
];

function isDownloadSpeedOutOfRange(value: number | null) {
  return value != null && (value <= 0 || value >= DOWNLOAD_SPEED_MAX_MBPS);
}

function languageLabel(code: string | null) {
  if (code === null) return "No preference";
  if (code === SUBTITLE_OFF_VALUE) return "Always off";
  return LANGUAGE_NAMES[code] ?? code;
}

type PreferenceOption = {
  value: string;
  label: string;
};

type PreferenceSelectProps = {
  /** Kept for native form identification; see the Radix note below. */
  name: string;
  label: string;
  /** null renders as the "no selection" option. */
  value: string | null;
  noSelectionLabel: string;
  options: PreferenceOption[];
  onChange: (value: string | null) => void;
};

/**
 * The select shape all three device preferences share: a leading "no
 * selection" item that maps to null, then the options.
 */
function PreferenceSelect({
  name,
  label,
  value,
  noSelectionLabel,
  options,
  onChange,
}: PreferenceSelectProps) {
  const selectId = useId();

  return (
    <div className="grid max-w-md gap-2">
      <Label htmlFor={selectId}>{label}</Label>
      {/*
        Radix renders a visually-hidden aria-hidden <select name> for each of
        these for native form integration; they're never submitted (this form is
        JS-controlled). Chrome's Issues panel flags them "no label" — a
        false-positive on aria-hidden fields, not fixable without ejecting
        Radix. Keep the names so the fields stay identified.
      */}
      <Select
        name={name}
        value={value ?? NO_SELECTION_VALUE}
        onValueChange={next =>
          onChange(next === NO_SELECTION_VALUE ? null : next)
        }
      >
        <SelectTrigger id={selectId} className={SETTINGS_SELECT_TRIGGER_CLASS}>
          <SelectValue placeholder={noSelectionLabel} />
        </SelectTrigger>
        <SelectContent className={SETTINGS_SELECT_CONTENT_CLASS}>
          <SelectItem
            value={NO_SELECTION_VALUE}
            className={SETTINGS_SELECT_ITEM_CLASS}
          >
            {noSelectionLabel}
          </SelectItem>
          {options.map(option => (
            <SelectItem
              key={option.value}
              value={option.value}
              className={SETTINGS_SELECT_ITEM_CLASS}
            >
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

type DevicePlaybackCardsProps = {
  settings: PlaybackSettingsType;
  userId: number;
  isAdmin: boolean;
};

/**
 * The per-device half of Settings -> Playback. These preferences live in
 * localStorage and apply the moment they change, so there is no save bar -- the
 * confirmation is the live announcement instead. localStorage can still refuse
 * the write, in which case the announcement says so and the cards carry a
 * standing notice (docs/design-system.md §3.7).
 */
export default function DevicePlaybackCards({
  settings,
  userId,
  isAdmin,
}: DevicePlaybackCardsProps) {
  const downloadMbpsId = useId();
  const recommendationId = useId();
  const recommendationTitleId = useId();
  const deviceStatusId = useId();

  const prefs = useDevicePlaybackPreferences(userId);
  // The input keeps its own text so a half-typed "0." neither reaches storage
  // nor snaps back under the user; committed values flow from `prefs`.
  const [downloadText, setDownloadText] = useState<string | null>(null);
  const [announcement, setAnnouncement] = useState("");
  const [announcementKey, setAnnouncementKey] = useState(0);
  const [persistenceFailed, setPersistenceFailed] = useState(false);

  const announce = (message: string, persisted: boolean) => {
    setAnnouncement(
      `${message} ${persisted ? DEVICE_SAVED_SUFFIX : DEVICE_SESSION_ONLY_SUFFIX}`,
    );
    setAnnouncementKey(current => current + 1);
  };

  const write = (patch: Partial<DevicePlaybackPreferences>) => {
    const persisted = setDevicePlaybackPreferences(userId, patch);
    setPersistenceFailed(!persisted);
    return persisted;
  };

  const persist = (
    patch: Partial<DevicePlaybackPreferences>,
    message: string,
  ) => {
    announce(message, write(patch));
  };

  const pendingDownload =
    downloadText === null ? prefs.downloadMbps : parseMbpsInput(downloadText);
  const downloadInvalid = isDownloadSpeedOutOfRange(pendingDownload);

  const handleDownloadMbpsChange = (value: string) => {
    setDownloadText(value);
    const parsed = parseMbpsInput(value);
    if (isDownloadSpeedOutOfRange(parsed)) return;
    if (parsed === prefs.downloadMbps) return;

    write({ downloadMbps: parsed });
  };

  // The write happened on change; blur only confirms it, so the outcome comes
  // from the standing flag rather than a fresh return value.
  const handleDownloadMbpsCommit = () => {
    if (downloadInvalid) return;
    announce(
      pendingDownload === null
        ? "Download speed cleared."
        : `Download speed set to ${pendingDownload} Mbps.`,
      !persistenceFailed,
    );
  };

  const handleProfileChange = (preferredProfile: string | null) => {
    const label = preferredProfile
      ? (settings.profiles.find(p => p.id === preferredProfile)?.label ??
        preferredProfile)
      : "Use recommended";
    persist({ preferredProfile }, `Preferred profile set to ${label}.`);
  };

  const handleAudioLanguageChange = (preferredAudioLanguage: string | null) => {
    persist(
      { preferredAudioLanguage },
      `Preferred audio language set to ${languageLabel(preferredAudioLanguage)}.`,
    );
  };

  const handleSubtitleLanguageChange = (
    preferredSubtitleLanguage: string | null,
  ) => {
    persist(
      { preferredSubtitleLanguage },
      `Preferred subtitles set to ${languageLabel(preferredSubtitleLanguage)}.`,
    );
  };

  const recommendedId = recommendedProfileId(
    settings.profiles,
    downloadInvalid ? prefs.downloadMbps : pendingDownload,
    settings.server_upload_mbps,
  );
  const recommendedProfile = recommendedId
    ? settings.profiles.find(p => p.id === recommendedId)
    : null;

  const persistenceNotice = persistenceFailed ? (
    <p className="px-6 text-sm text-destructive">
      {DEVICE_PERSISTENCE_FAILED_MESSAGE}
    </p>
  ) : null;

  return (
    <>
      <LiveAnnouncer message={announcement} announcementKey={announcementKey} />
      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={Gauge}
          title="Streaming & bandwidth"
          description={`Tell Igloo about this connection so it can pick the right stream quality. ${DEVICE_SCOPE_DESCRIPTION}`}
        />
        {persistenceNotice}
        <CardContent className="divide-y divide-border/50">
          <PlaybackSection
            title="Your network speed"
            description="Used to recommend a stream profile for this device."
          >
            <div className="grid max-w-md gap-2">
              <Label htmlFor={downloadMbpsId}>Download speed (Mbps)</Label>
              <Input
                id={downloadMbpsId}
                name="download_mbps"
                type="number"
                inputMode="decimal"
                min={0.1}
                step={0.1}
                value={downloadText ?? prefs.downloadMbps ?? ""}
                onChange={event => handleDownloadMbpsChange(event.target.value)}
                onBlur={handleDownloadMbpsCommit}
                aria-invalid={downloadInvalid ? "true" : undefined}
                aria-describedby={
                  downloadInvalid
                    ? `${downloadMbpsId}-description ${deviceStatusId}`
                    : `${downloadMbpsId}-description`
                }
                className={SETTINGS_INPUT_CLASS}
              />
              <p
                id={`${downloadMbpsId}-description`}
                className="text-sm text-muted-foreground"
              >
                Leave blank if unsure.
              </p>
              <p
                id={deviceStatusId}
                aria-live="polite"
                className="text-sm text-destructive empty:hidden"
              >
                {downloadInvalid ? DOWNLOAD_SPEED_VALIDATION_MESSAGE : ""}
              </p>
            </div>
          </PlaybackSection>
          <PlaybackSection
            title="Server upload cap"
            description="The home server's outbound bandwidth limit, applied when you stream from outside the home network."
          >
            <p className="text-sm text-foreground">
              {settings.server_upload_mbps != null
                ? `${settings.server_upload_mbps} Mbps`
                : "Not set (uncapped)"}
            </p>
            <p className="mt-2 text-sm text-muted-foreground">
              {isAdmin
                ? "Set for the whole server below. Affects your recommendation when streaming from outside the home network."
                : "Set by the server administrator. Affects your recommendation when streaming from outside the home network."}
            </p>
          </PlaybackSection>
          <PlaybackSection
            title="Recommended profile"
            titleId={recommendationTitleId}
            description="Calculated from your download speed and the server upload cap, with 20% headroom for audio and overhead."
          >
            <p
              id={recommendationId}
              role="region"
              aria-labelledby={recommendationTitleId}
              aria-live="polite"
              className="text-sm text-foreground"
            >
              {recommendedProfile
                ? `Recommended: ${recommendedProfile.label}`
                : "Enter your download speed to see a recommendation."}
            </p>
          </PlaybackSection>
        </CardContent>
      </Card>

      <Card
        className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
      >
        <SettingsCardHeader
          icon={Sliders}
          title="Stream defaults"
          description={`Defaults applied when you start a stream; you can still change each per video. ${DEVICE_SCOPE_DESCRIPTION}`}
        />
        {persistenceNotice}
        <CardContent className="divide-y divide-border/50">
          <PlaybackSection
            title="Preferred profile"
            description="The default profile when you start a stream."
          >
            <PreferenceSelect
              name="preferred_profile"
              label="Profile"
              value={prefs.preferredProfile}
              noSelectionLabel="Use recommended"
              options={settings.profiles.map(profile => ({
                value: profile.id,
                label: profile.label,
              }))}
              onChange={handleProfileChange}
            />
          </PlaybackSection>
          <PlaybackSection
            title="Preferred audio language"
            description="Igloo will pick the matching audio track when a movie has one."
          >
            <PreferenceSelect
              name="preferred_audio_language"
              label="Audio language"
              value={prefs.preferredAudioLanguage}
              noSelectionLabel="No preference"
              options={LANGUAGE_OPTIONS}
              onChange={handleAudioLanguageChange}
            />
          </PlaybackSection>
          <PlaybackSection
            title="Preferred subtitles"
            description="Pick a default subtitle language, or always start with subtitles off."
          >
            <PreferenceSelect
              name="preferred_subtitle_language"
              label="Subtitle language"
              value={prefs.preferredSubtitleLanguage}
              noSelectionLabel="No preference"
              options={SUBTITLE_OPTIONS}
              onChange={handleSubtitleLanguageChange}
            />
          </PlaybackSection>
        </CardContent>
      </Card>
    </>
  );
}
