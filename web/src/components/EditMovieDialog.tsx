import { useEffect, useReducer, useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  identifyMovie,
  updateMovieMetadata,
} from "@/lib/api";
import {
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_MOVIE_DETAILS_KEY,
  MOTION_MICRO_CONTROL_CLASS,
  MOVIE_TECHNICAL_DETAILS_KEY,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import { unwrapInt, unwrapString } from "@/lib/nullable";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import TmdbMoviePicker from "@/components/TmdbMoviePicker";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import type {
  LibraryMovieDetailsMovieType,
  UpdateMovieMetadataRequest,
} from "@/types";

type Props = {
  movieId: number;
  movie: LibraryMovieDetailsMovieType;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

const MANUAL_OVERVIEW_LABEL_ID = "manual-overview-label";

const movieMetadataFields = [
  "title",
  "year",
  "releaseDate",
  "overview",
  "tagLine",
  "certification",
  "posterPath",
  "backdropPath",
  "language",
] as const;

type MovieMetadataField = (typeof movieMetadataFields)[number];

type MovieMetadataForm = Record<MovieMetadataField, string>;

type MovieMetadataDirtyFields = Record<MovieMetadataField, boolean>;

type MovieMetadataFormState = {
  baseline: MovieMetadataForm;
  draft: MovieMetadataForm;
  dirty: MovieMetadataDirtyFields;
};

type MovieMetadataFormAction =
  | {
      type: "field";
      field: MovieMetadataField;
      value: string;
    }
  | {
      type: "source";
      baseline: MovieMetadataForm;
    };

function buildMovieMetadataForm(
  movie: LibraryMovieDetailsMovieType,
): MovieMetadataForm {
  return {
    title: movie.title,
    year: String(unwrapInt(movie.year) ?? ""),
    releaseDate: unwrapString(movie.release_date) ?? "",
    overview: unwrapString(movie.overview) ?? "",
    tagLine: unwrapString(movie.tag_line) ?? "",
    certification: unwrapString(movie.certification) ?? "",
    posterPath: unwrapString(movie.poster_path) ?? "",
    backdropPath: unwrapString(movie.backdrop_path) ?? "",
    language: unwrapString(movie.language) ?? "",
  };
}

function movieMetadataFormsEqual(
  first: MovieMetadataForm,
  second: MovieMetadataForm,
) {
  return movieMetadataFields.every((field) => first[field] === second[field]);
}

function buildDirtyFields(
  draft: MovieMetadataForm,
  baseline: MovieMetadataForm,
): MovieMetadataDirtyFields {
  return movieMetadataFields.reduce((dirty, field) => {
    dirty[field] = draft[field] !== baseline[field];
    return dirty;
  }, {} as MovieMetadataDirtyFields);
}

function mergeBackingMetadataIntoDraft(
  draft: MovieMetadataForm,
  dirty: MovieMetadataDirtyFields,
  nextBaseline: MovieMetadataForm,
): MovieMetadataForm {
  return movieMetadataFields.reduce((nextDraft, field) => {
    nextDraft[field] = dirty[field] ? draft[field] : nextBaseline[field];
    return nextDraft;
  }, {} as MovieMetadataForm);
}

function createMovieMetadataFormState(
  movie: LibraryMovieDetailsMovieType,
): MovieMetadataFormState {
  const baseline = buildMovieMetadataForm(movie);
  const draft = { ...baseline };

  return {
    baseline,
    draft,
    dirty: buildDirtyFields(draft, baseline),
  };
}

function movieMetadataFormReducer(
  state: MovieMetadataFormState,
  action: MovieMetadataFormAction,
): MovieMetadataFormState {
  if (action.type === "field") {
    const draft = {
      ...state.draft,
      [action.field]: action.value,
    };
    const dirty = {
      ...state.dirty,
      [action.field]: action.value !== state.baseline[action.field],
    };

    return {
      ...state,
      draft,
      dirty,
    };
  }

  if (movieMetadataFormsEqual(state.baseline, action.baseline)) {
    return state;
  }

  const draft = mergeBackingMetadataIntoDraft(
    state.draft,
    state.dirty,
    action.baseline,
  );

  return {
    baseline: action.baseline,
    draft,
    dirty: buildDirtyFields(draft, action.baseline),
  };
}

function buildUpdateMovieMetadataRequest(
  draft: MovieMetadataForm,
  baseline: MovieMetadataForm,
): UpdateMovieMetadataRequest {
  const body: UpdateMovieMetadataRequest = {};

  if (draft.title !== baseline.title) body.title = draft.title;

  const year = parseInt(draft.year, 10);
  const baselineYear = parseInt(baseline.year, 10);
  if (year > 0 && year !== baselineYear) body.year = year;
  else if (!draft.year.trim() && baseline.year.trim()) body.year = 0;

  if (draft.releaseDate !== baseline.releaseDate)
    body.release_date = draft.releaseDate;
  if (draft.overview !== baseline.overview) body.overview = draft.overview;
  if (draft.tagLine !== baseline.tagLine) body.tag_line = draft.tagLine;
  if (draft.certification !== baseline.certification)
    body.certification = draft.certification;
  if (draft.posterPath !== baseline.posterPath)
    body.poster_path = draft.posterPath;
  if (draft.backdropPath !== baseline.backdropPath)
    body.backdrop_path = draft.backdropPath;
  if (draft.language !== baseline.language) body.language = draft.language;

  return body;
}

export default function EditMovieDialog({
  movieId,
  movie,
  open,
  onOpenChange,
  restoreFocusRef,
}: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[85vh] overflow-y-auto border-border bg-card sm:max-w-2xl"
        onCloseAutoFocus={(event) => {
          if (!restoreFocusRef) return;
          event.preventDefault();
          focusDialogRestoreTarget(restoreFocusRef.current);
        }}
      >
        <DialogTitle className="text-foreground">Edit Movie</DialogTitle>
        <DialogDescription className="text-muted-foreground">
          Identify with TMDB to replace all metadata, or manually edit
          individual fields.
        </DialogDescription>

        <Tabs defaultValue="tmdb">
          <TabsList className="w-full">
            <TabsTrigger value="tmdb" className="flex-1">
              Identify with TMDB
            </TabsTrigger>
            <TabsTrigger value="manual" className="flex-1">
              Manual
            </TabsTrigger>
          </TabsList>

          <TabsContent key={`tmdb-${movieId}`} value="tmdb">
            <TmdbTab movieId={movieId} movie={movie} onOpenChange={onOpenChange} />
          </TabsContent>

          <TabsContent key={`manual-${movieId}`} value="manual">
            <ManualTab movieId={movieId} movie={movie} onOpenChange={onOpenChange} />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}

function TmdbTab({
  movieId,
  movie,
  onOpenChange,
}: {
  movieId: number;
  movie: LibraryMovieDetailsMovieType;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();

  async function handleApply(selectedId: number) {
    const res = await identifyMovie(movieId, selectedId);
    if (res.error) {
      toast.error(res.message || "Failed to identify movie");
      return;
    }

    queryClient.invalidateQueries({
      queryKey: [LIBRARY_MOVIE_DETAILS_KEY, movieId],
    });
    queryClient.invalidateQueries({
      queryKey: [MOVIE_TECHNICAL_DETAILS_KEY, movieId],
    });
    toast.success("Movie identified successfully");
    onOpenChange(false);
  }

  return (
    <TmdbMoviePicker
      confirmLabel="Apply Selected"
      initialTitle={movie.title}
      initialYear={String(unwrapInt(movie.year) ?? "")}
      initialTmdbId={String(unwrapInt(movie.tmdb_id) ?? "")}
      onConfirm={async selectedResult => handleApply(selectedResult.tmdb_id)}
      showTmdbIdInput
    />
  );
}

function ManualTab({
  movieId,
  movie,
  onOpenChange,
}: {
  movieId: number;
  movie: LibraryMovieDetailsMovieType;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [saving, setSaving] = useState(false);
  const [form, dispatchForm] = useReducer(
    movieMetadataFormReducer,
    movie,
    createMovieMetadataFormState,
  );

  useEffect(() => {
    dispatchForm({
      type: "source",
      baseline: buildMovieMetadataForm(movie),
    });
  }, [movie]);

  const { draft, baseline } = form;
  async function handleSave() {
    setSaving(true);

    const body = buildUpdateMovieMetadataRequest(draft, baseline);

    if (Object.keys(body).length === 0) {
      toast.info("No changes to save");
      setSaving(false);
      return;
    }

    const res = await updateMovieMetadata(movieId, body);
    setSaving(false);

    if (res.error) {
      toast.error(res.message || "Failed to update movie");
      return;
    }

    queryClient.invalidateQueries({
      queryKey: [LIBRARY_MOVIE_DETAILS_KEY, movieId],
    });
    queryClient.invalidateQueries({
      queryKey: [MOVIE_TECHNICAL_DETAILS_KEY, movieId],
    });
    toast.success("Movie updated successfully");
    onOpenChange(false);
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2">
        <FieldGroup label="Title" htmlFor="manual-title">
          <Input
            id="manual-title"
            value={draft.title}
            onChange={(e) =>
              dispatchForm({
                type: "field",
                field: "title",
                value: e.target.value,
              })
            }
            className="border-border bg-muted text-foreground"
          />
        </FieldGroup>
        <FieldGroup label="Year" htmlFor="manual-year">
          <Input
            id="manual-year"
            type="number"
            value={draft.year}
            onChange={(e) =>
              dispatchForm({
                type: "field",
                field: "year",
                value: e.target.value,
              })
            }
            className="border-border bg-muted text-foreground"
          />
        </FieldGroup>
        <FieldGroup label="Release Date" htmlFor="manual-release-date">
          <Input
            id="manual-release-date"
            type="date"
            value={draft.releaseDate}
            onChange={(e) =>
              dispatchForm({
                type: "field",
                field: "releaseDate",
                value: e.target.value,
              })
            }
            className="border-border bg-muted text-foreground"
          />
        </FieldGroup>
        <FieldGroup label="Certification" htmlFor="manual-certification">
          <Input
            id="manual-certification"
            value={draft.certification}
            onChange={(e) =>
              dispatchForm({
                type: "field",
                field: "certification",
                value: e.target.value,
              })
            }
            placeholder="e.g. PG-13, R"
            className="border-border bg-muted text-foreground"
          />
        </FieldGroup>
        <FieldGroup label="Language" htmlFor="manual-language">
          <Input
            id="manual-language"
            value={draft.language}
            onChange={(e) =>
              dispatchForm({
                type: "field",
                field: "language",
                value: e.target.value,
              })
            }
            placeholder="e.g. en"
            className="border-border bg-muted text-foreground"
          />
        </FieldGroup>
        <FieldGroup label="Tagline" htmlFor="manual-tagline">
          <Input
            id="manual-tagline"
            value={draft.tagLine}
            onChange={(e) =>
              dispatchForm({
                type: "field",
                field: "tagLine",
                value: e.target.value,
              })
            }
            className="border-border bg-muted text-foreground"
          />
        </FieldGroup>
        <FieldGroup label="Poster Path" htmlFor="manual-poster">
          <Input
            id="manual-poster"
            value={draft.posterPath}
            onChange={(e) =>
              dispatchForm({
                type: "field",
                field: "posterPath",
                value: e.target.value,
              })
            }
            placeholder="/abcdef.jpg"
            className="border-border bg-muted text-foreground"
          />
        </FieldGroup>
        <FieldGroup label="Backdrop Path" htmlFor="manual-backdrop">
          <Input
            id="manual-backdrop"
            value={draft.backdropPath}
            onChange={(e) =>
              dispatchForm({
                type: "field",
                field: "backdropPath",
                value: e.target.value,
              })
            }
            placeholder="/abcdef.jpg"
            className="border-border bg-muted text-foreground"
          />
        </FieldGroup>
      </div>

      <FieldGroup
        label="Overview"
        htmlFor="manual-overview"
        labelId={MANUAL_OVERVIEW_LABEL_ID}
      >
        <textarea
          id="manual-overview"
          aria-labelledby={MANUAL_OVERVIEW_LABEL_ID}
          value={draft.overview}
          onChange={(e) =>
            dispatchForm({
              type: "field",
              field: "overview",
              value: e.target.value,
            })
          }
          rows={4}
          className={cn(
            MOTION_MICRO_CONTROL_CLASS,
            FOCUS_VISIBLE_RING_CLASS,
            "w-full rounded-md border border-border bg-muted px-3 py-2 text-sm text-foreground",
          )}
        />
      </FieldGroup>

      <Button
        onClick={handleSave}
        disabled={saving}
        variant="accent"
        className="w-full"
      >
        {saving && <Spinner className="size-4" />}
        {saving ? "Saving…" : "Save Changes"}
      </Button>
    </div>
  );
}

function FieldGroup({
  label,
  htmlFor,
  labelId,
  children,
}: {
  label: string;
  htmlFor: string;
  labelId?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <Label id={labelId} htmlFor={htmlFor} className="text-muted-foreground">
        {label}
      </Label>
      <div className="mt-1">{children}</div>
    </div>
  );
}
