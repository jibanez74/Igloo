import { useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Film, Search } from "lucide-react";
import { toast } from "sonner";
import {
  tmdbSearchMovies,
  identifyMovie,
  updateMovieMetadata,
} from "@/lib/api";
import {
  LIBRARY_MOVIE_DETAILS_KEY,
  MOVIE_TECHNICAL_DETAILS_KEY,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { unwrapInt, unwrapString } from "@/lib/nullable";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type {
  LibraryMovieDetailsMovieType,
  TmdbSearchResultType,
  UpdateMovieMetadataRequest,
} from "@/types";

type Props = {
  movieId: number;
  movie: LibraryMovieDetailsMovieType;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

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
          const restoreTarget = restoreFocusRef?.current;
          if (!restoreTarget) return;

          event.preventDefault();
          restoreTarget.focus();
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

          <TabsContent value="tmdb">
            <TmdbTab movieId={movieId} movie={movie} onOpenChange={onOpenChange} />
          </TabsContent>

          <TabsContent value="manual">
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

  const [title, setTitle] = useState(movie.title);
  const [year, setYear] = useState(String(unwrapInt(movie.year) ?? ""));
  const [tmdbIdInput, setTmdbIdInput] = useState(
    String(unwrapInt(movie.tmdb_id) ?? ""),
  );
  const [results, setResults] = useState<TmdbSearchResultType[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searching, setSearching] = useState(false);
  const [applying, setApplying] = useState(false);

  async function handleSearch() {
    if (!title.trim() && !tmdbIdInput.trim()) return;
    setSearching(true);
    setResults([]);
    setSelectedId(null);

    const body: { title: string; year?: number; tmdb_id?: number } = {
      title: title.trim(),
    };
    const y = parseInt(year, 10);
    if (y > 0) body.year = y;
    const tid = parseInt(tmdbIdInput, 10);
    if (tid > 0) body.tmdb_id = tid;

    const res = await tmdbSearchMovies(movieId, body);
    setSearching(false);

    if (res.error || !res.data?.results) {
      toast.error(res.message || "TMDB search failed");
      return;
    }
    setResults(res.data.results);
    if (res.data.results.length === 0) {
      toast.info("No results found");
    }
  }

  async function handleApply() {
    if (selectedId == null) return;
    setApplying(true);

    const res = await identifyMovie(movieId, selectedId);
    setApplying(false);

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
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <div className="sm:col-span-1">
          <Label htmlFor="tmdb-title" className="text-slate-300">
            Title
          </Label>
          <Input
            id="tmdb-title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="mt-1 border-slate-700 bg-slate-800 text-white"
          />
        </div>
        <div>
          <Label htmlFor="tmdb-year" className="text-slate-300">
            Year
          </Label>
          <Input
            id="tmdb-year"
            type="number"
            value={year}
            onChange={(e) => setYear(e.target.value)}
            className="mt-1 border-slate-700 bg-slate-800 text-white"
          />
        </div>
        <div>
          <Label htmlFor="tmdb-id" className="text-slate-300">
            TMDB ID
          </Label>
          <Input
            id="tmdb-id"
            type="number"
            value={tmdbIdInput}
            onChange={(e) => setTmdbIdInput(e.target.value)}
            placeholder="Optional"
            className="mt-1 border-slate-700 bg-slate-800 text-white"
          />
        </div>
      </div>

      <Button
        onClick={handleSearch}
        disabled={searching || (!title.trim() && !tmdbIdInput.trim())}
        className="w-full"
      >
        {searching ? (
          <Spinner className="size-4" />
        ) : (
          <Search className="size-4" aria-hidden="true" />
        )}
        {searching ? "Searching…" : "Search TMDB"}
      </Button>

      {results.length > 0 && (
        <div className="space-y-2">
          <p className="text-sm text-slate-400">
            {results.length} result{results.length !== 1 && "s"} — select one
            to apply
          </p>

          <ul className="max-h-64 space-y-2 overflow-y-auto" role="listbox">
            {results.map((r) => (
              <TmdbResultCard
                key={r.tmdb_id}
                result={r}
                selected={selectedId === r.tmdb_id}
                onSelect={() => setSelectedId(r.tmdb_id)}
              />
            ))}
          </ul>

          <Button
            onClick={handleApply}
            disabled={selectedId == null || applying}
            variant="accent"
            className="w-full"
          >
            {applying && <Spinner className="size-4" />}
            {applying ? "Applying…" : "Apply Selected"}
          </Button>
        </div>
      )}
    </div>
  );
}

function TmdbResultCard({
  result,
  selected,
  onSelect,
}: {
  result: TmdbSearchResultType;
  selected: boolean;
  onSelect: () => void;
}) {
  const posterUrl = result.poster_path
    ? buildTmdbImageUrl(result.poster_path, TMDB_POSTER_SIZE)
    : null;

  const releaseYear = result.release_date?.slice(0, 4);

  return (
    <li
      role="option"
      aria-selected={selected}
      tabIndex={0}
      onClick={onSelect}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onSelect();
        }
      }}
      className={`flex cursor-pointer gap-3 rounded-lg border p-2 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-amber-500/60 ${
        selected
          ? "border-amber-500 bg-amber-500/10"
          : "border-slate-700 bg-slate-800/60 hover:border-slate-600"
      }`}
    >
      {posterUrl ? (
        <img
          src={posterUrl}
          alt=""
          className="h-20 w-14 shrink-0 rounded-sm object-cover"
        />
      ) : (
        <div className="flex h-20 w-14 shrink-0 items-center justify-center rounded-sm bg-slate-700">
          <Film className="size-5 text-slate-500" aria-hidden="true" />
        </div>
      )}
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium text-white">
          {result.title}
          {releaseYear && (
            <span className="ml-1 text-slate-400">({releaseYear})</span>
          )}
        </p>
        <p className="mt-0.5 text-xs text-slate-500">
          TMDB ID: {result.tmdb_id}
        </p>
        {result.overview && (
          <p className="mt-1 line-clamp-2 text-sm text-slate-400">
            {result.overview}
          </p>
        )}
      </div>
    </li>
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

  const [title, setTitle] = useState(movie.title);
  const [year, setYear] = useState(String(unwrapInt(movie.year) ?? ""));
  const [releaseDate, setReleaseDate] = useState(
    unwrapString(movie.release_date) ?? "",
  );
  const [overview, setOverview] = useState(
    unwrapString(movie.overview) ?? "",
  );
  const [tagLine, setTagLine] = useState(unwrapString(movie.tag_line) ?? "");
  const [certification, setCertification] = useState(
    unwrapString(movie.certification) ?? "",
  );
  const [posterPath, setPosterPath] = useState(
    unwrapString(movie.poster_path) ?? "",
  );
  const [backdropPath, setBackdropPath] = useState(
    unwrapString(movie.backdrop_path) ?? "",
  );
  const [language, setLanguage] = useState(
    unwrapString(movie.language) ?? "",
  );

  async function handleSave() {
    setSaving(true);

    const body: UpdateMovieMetadataRequest = {};
    if (title !== movie.title) body.title = title;

    const y = parseInt(year, 10);
    const origYear = unwrapInt(movie.year);
    if (y > 0 && y !== origYear) body.year = y;
    else if (!year.trim() && origYear != null) body.year = 0;

    const orig = (fn: typeof unwrapString, field: typeof movie.release_date) =>
      fn(field) ?? "";

    if (releaseDate !== orig(unwrapString, movie.release_date))
      body.release_date = releaseDate;
    if (overview !== orig(unwrapString, movie.overview))
      body.overview = overview;
    if (tagLine !== orig(unwrapString, movie.tag_line))
      body.tag_line = tagLine;
    if (certification !== orig(unwrapString, movie.certification))
      body.certification = certification;
    if (posterPath !== orig(unwrapString, movie.poster_path))
      body.poster_path = posterPath;
    if (backdropPath !== orig(unwrapString, movie.backdrop_path))
      body.backdrop_path = backdropPath;
    if (language !== orig(unwrapString, movie.language))
      body.language = language;

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
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
        <FieldGroup label="Year" htmlFor="manual-year">
          <Input
            id="manual-year"
            type="number"
            value={year}
            onChange={(e) => setYear(e.target.value)}
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
        <FieldGroup label="Release Date" htmlFor="manual-release-date">
          <Input
            id="manual-release-date"
            type="date"
            value={releaseDate}
            onChange={(e) => setReleaseDate(e.target.value)}
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
        <FieldGroup label="Certification" htmlFor="manual-certification">
          <Input
            id="manual-certification"
            value={certification}
            onChange={(e) => setCertification(e.target.value)}
            placeholder="e.g. PG-13, R"
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
        <FieldGroup label="Language" htmlFor="manual-language">
          <Input
            id="manual-language"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            placeholder="e.g. en"
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
        <FieldGroup label="Tagline" htmlFor="manual-tagline">
          <Input
            id="manual-tagline"
            value={tagLine}
            onChange={(e) => setTagLine(e.target.value)}
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
        <FieldGroup label="Poster Path" htmlFor="manual-poster">
          <Input
            id="manual-poster"
            value={posterPath}
            onChange={(e) => setPosterPath(e.target.value)}
            placeholder="/abcdef.jpg"
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
        <FieldGroup label="Backdrop Path" htmlFor="manual-backdrop">
          <Input
            id="manual-backdrop"
            value={backdropPath}
            onChange={(e) => setBackdropPath(e.target.value)}
            placeholder="/abcdef.jpg"
            className="border-slate-700 bg-slate-800 text-white"
          />
        </FieldGroup>
      </div>

      <FieldGroup label="Overview" htmlFor="manual-overview">
        <textarea
          id="manual-overview"
          value={overview}
          onChange={(e) => setOverview(e.target.value)}
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
  children,
}: {
  label: string;
  htmlFor: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <Label htmlFor={htmlFor} className="text-slate-300">
        {label}
      </Label>
      <div className="mt-1">{children}</div>
    </div>
  );
}
