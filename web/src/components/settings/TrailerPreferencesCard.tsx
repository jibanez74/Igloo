import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Clapperboard } from "lucide-react";
import { useId, useState } from "react";

import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import SettingsErrorCard from "@/components/settings/SettingsErrorCard";
import SettingsLoadingCard from "@/components/settings/SettingsLoadingCard";
import SwitchField from "@/components/settings/SwitchField";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { updateTrailerPreferences } from "@/lib/api";
import {
  MOTION_SETTINGS_SURFACE_CLASS,
  SETTINGS_CARD_SURFACE_CLASS,
  SETTINGS_SELECT_CONTENT_CLASS,
  SETTINGS_SELECT_ITEM_CLASS,
  SETTINGS_SELECT_TRIGGER_CLASS,
  TRAILER_COUNT_OPTIONS,
  TRAILER_PREFERENCES_KEY,
  TRAILER_SOURCE_OPTIONS,
} from "@/lib/constants";
import {
  tmdbStatusQueryOpts,
  trailerPreferencesQueryOpts,
} from "@/lib/query-opts";
import { showActionFailed } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  ApiResponseType,
  TrailerPreferencesData,
  TrailerSource,
} from "@/types";

/** Explains that this card writes to the account rather than the browser. */
const ACCOUNT_SCOPE_DESCRIPTION =
  "Play a few trailers before a movie starts from the beginning, like a cinema. Saved to your account, so it follows you across browsers and devices.";
const ACCOUNT_SAVED_SUFFIX = "Saved to your account.";
const TMDB_UNAVAILABLE_HINT =
  "In theaters trailers are unavailable because TMDB is not configured on this server. Library trailers will be used instead.";

type TrailerPreferencesQueryData = ApiResponseType<TrailerPreferencesData>;

type SaveVariables = {
  prefs: TrailerPreferencesData;
  /** What changed, spoken once the save lands. */
  announcement: string;
};

function sourceLabel(source: TrailerSource) {
  return (
    TRAILER_SOURCE_OPTIONS.find((option) => option.value === source)?.label ??
    source
  );
}

function isTrailerSource(value: string): value is TrailerSource {
  return TRAILER_SOURCE_OPTIONS.some((option) => option.value === value);
}

/**
 * Settings → Playback, "Trailers before movies": the account-scoped pre-roll
 * preferences (design-system §3.7). Every change saves at once with an
 * optimistic update; LiveAnnouncer confirms the save, a failure toasts and
 * rolls the control back.
 */
export default function TrailerPreferencesCard() {
  const queryClient = useQueryClient();
  const { data, isPending } = useQuery(trailerPreferencesQueryOpts());
  const { data: tmdbStatusData } = useQuery(tmdbStatusQueryOpts());
  const [announcement, setAnnouncement] = useState("");
  const [announcementKey, setAnnouncementKey] = useState(0);
  const switchId = useId();
  const countId = useId();
  const sourceId = useId();
  const sourceHintId = useId();

  const saveMutation = useMutation({
    mutationFn: ({ prefs }: SaveVariables) => updateTrailerPreferences(prefs),
    onMutate: async ({ prefs }) => {
      await queryClient.cancelQueries({ queryKey: [TRAILER_PREFERENCES_KEY] });
      const previousData =
        queryClient.getQueryData<TrailerPreferencesQueryData>([
          TRAILER_PREFERENCES_KEY,
        ]);
      queryClient.setQueryData<TrailerPreferencesQueryData>(
        [TRAILER_PREFERENCES_KEY],
        { error: false, data: prefs },
      );
      return { previousData };
    },
    onSuccess: (res, variables, context) => {
      if (res.error) {
        if (context?.previousData) {
          queryClient.setQueryData(
            [TRAILER_PREFERENCES_KEY],
            context.previousData,
          );
        }
        showActionFailed("save trailer preferences", res.message);
        return;
      }
      queryClient.setQueryData<TrailerPreferencesQueryData>(
        [TRAILER_PREFERENCES_KEY],
        { error: false, message: res.message, data: res.data },
      );
      setAnnouncement(`${variables.announcement} ${ACCOUNT_SAVED_SUFFIX}`);
      setAnnouncementKey((current) => current + 1);
    },
    onError: (_error, _variables, context) => {
      if (context?.previousData) {
        queryClient.setQueryData(
          [TRAILER_PREFERENCES_KEY],
          context.previousData,
        );
      }
      showActionFailed(
        "save trailer preferences",
        "An unexpected error occurred",
      );
    },
  });

  if (isPending) {
    return <SettingsLoadingCard label="Loading trailer preferences..." />;
  }

  if (!data || data.error) {
    return (
      <SettingsErrorCard
        title="Trailer preferences unavailable"
        message={data?.message || "Failed to load trailer preferences."}
      />
    );
  }

  const prefs = data.data;
  const saving = saveMutation.isPending;
  const tmdbUnavailable =
    tmdbStatusData?.error === false && !tmdbStatusData.data.available;

  const save = (next: TrailerPreferencesData, announcement: string) => {
    saveMutation.mutate({ prefs: next, announcement });
  };

  return (
    <Card
      className={cn(SETTINGS_CARD_SURFACE_CLASS, MOTION_SETTINGS_SURFACE_CLASS)}
    >
      <SettingsCardHeader
        icon={Clapperboard}
        title="Trailers before movies"
        description={ACCOUNT_SCOPE_DESCRIPTION}
      />
      <LiveAnnouncer message={announcement} announcementKey={announcementKey} />
      <CardContent className="space-y-5">
        <SwitchField
          id={switchId}
          label="Play trailers before movies"
          description="Only when a movie starts from the beginning; resuming skips straight to the movie."
          checked={prefs.enabled}
          disabled={saving}
          onCheckedChange={(checked) =>
            save(
              { ...prefs, enabled: checked },
              `Trailers before movies turned ${checked ? "on" : "off"}.`,
            )
          }
          icon={
            <Clapperboard className="size-5 text-primary" aria-hidden="true" />
          }
        />

        <div className="grid gap-5 sm:grid-cols-2">
          <div className="grid gap-2">
            <Label htmlFor={countId}>Number of trailers</Label>
            <Select
              name="trailer_count"
              value={String(prefs.count)}
              disabled={saving}
              onValueChange={(value) => {
                const count = Number(value);
                save({ ...prefs, count }, `Trailer count set to ${count}.`);
              }}
            >
              <SelectTrigger
                id={countId}
                className={SETTINGS_SELECT_TRIGGER_CLASS}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent className={SETTINGS_SELECT_CONTENT_CLASS}>
                {TRAILER_COUNT_OPTIONS.map((count) => (
                  <SelectItem
                    key={count}
                    value={String(count)}
                    className={SETTINGS_SELECT_ITEM_CLASS}
                  >
                    {count === 1 ? "1 trailer" : `${count} trailers`}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid gap-2">
            <Label htmlFor={sourceId}>Trailer source</Label>
            <Select
              name="trailer_source"
              value={prefs.source}
              disabled={saving}
              onValueChange={(value) => {
                if (!isTrailerSource(value)) return;
                save(
                  { ...prefs, source: value },
                  `Trailer source set to ${sourceLabel(value)}.`,
                );
              }}
            >
              <SelectTrigger
                id={sourceId}
                className={SETTINGS_SELECT_TRIGGER_CLASS}
                aria-describedby={tmdbUnavailable ? sourceHintId : undefined}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent className={SETTINGS_SELECT_CONTENT_CLASS}>
                {TRAILER_SOURCE_OPTIONS.map((option) => (
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
            {tmdbUnavailable && (
              <p id={sourceHintId} className="text-xs text-muted-foreground">
                {TMDB_UNAVAILABLE_HINT}
              </p>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
