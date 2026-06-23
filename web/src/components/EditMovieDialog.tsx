import { useEffect, useReducer, useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  identifyMovie,
  updateMovieMetadata,
} from "@/lib/api";
import {
  LIBRARY_MOVIE_DETAILS_KEY,
  MOVIE_TECHNICAL_DETAILS_KEY,
} from "@/lib/constants";
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
        className="max-h-[85vh] overflow-y-auto border-slate-700 bg-slate-900 sm:max-w-2xl"
        onCloseAutoFocus={(event) => {
          if (!restoreFocusRef) return;
          event.preventDefault();
          focusDialogRestoreTarget(restoreFocusRef.current);
        }}
      >
        <DialogTitle className="text-white">Edit Movie</DialogTitle>
        <DialogDescription className="text-slate-400">
          Identify with TMDB to replace all metadata, or manually edit
          individual fields.
        </DialogDescription>

        <Tabs defaultValue="tmdb">
          <TabsList className="w-full bg-slate-800">
            <TabsTrigger
              value="tmdb"
              className="flex-1 text-slate-300 data-[state=active]:bg-slate-700 data-[state=active]:text-white"
            >
              Identify with TMDB
            </TabsTrigger>
            <TabsTrigger
              value="manual"
              className="flex-1 text-slate-300 data-[state=active]:bg-slate-700 data-[state=active]:text-white"
            >
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
  const overviewLabelId = "manual-overview-label";

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
            className="border-slate-700 bg-slate-800 text-white"
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
            className="border-slate-700 bg-slate-800 text-white"
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
            className="border-slate-700 bg-slate-800 text-white"
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
            className="border-slate-700 bg-slate-800 text-white"
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
            className="border-slate-700 bg-slate-800 text-white"
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
            className="border-slate-700 bg-slate-800 text-white"
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
            className="border-slate-700 bg-slate-800 text-white"
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
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
      </div>

      <FieldGroup
        label="Overview"
        htmlFor="manual-overview"
        labelId={overviewLabelId}
      >
        <textarea
          id="manual-overview"
          aria-labelledby={overviewLabelId}
          value={draft.overview}
          onChange={(e) =>
            dispatchForm({
              type: "field",
              field: "overview",
              value: e.target.value,
            })
          }
          rows={4}
          className="w-full rounded-md border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white transition-shadow outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
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
      <Label id={labelId} htmlFor={htmlFor} className="text-slate-300">
        {label}
      </Label>
      <div className="mt-1">{children}</div>
    </div>
  );
}
