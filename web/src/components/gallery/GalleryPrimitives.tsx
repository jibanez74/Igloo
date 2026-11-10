import {
  Activity,
  AlertCircle,
  Bell,
  Film,
  Heart,
  ListMusic,
  Play,
  Plus,
  RotateCcw,
  Shuffle,
  Trash2,
} from "lucide-react";
import { GalleryRow, GallerySection } from "@/components/gallery/GallerySection";
import SwitchField from "@/components/settings/SwitchField";
import LibraryPagination from "@/components/shared/LibraryPagination";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
  MEDIA_FALLBACK_SURFACE_CLASS,
  OVER_MEDIA_BADGE_CLASS,
  SETTINGS_CARD_SURFACE_CLASS,
  SETTINGS_RESET_BUTTON_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

const BUTTON_VARIANT_NAMES = [
  "default",
  "accent",
  "accent-pill",
  "outline",
  "ghost",
  "destructive",
] as const;

const noop = () => {};

function ButtonsSection() {
  return (
    <GallerySection id="buttons" title="Button">
      {BUTTON_VARIANT_NAMES.map(variant => (
        <GalleryRow key={variant} label={variant}>
          <Button variant={variant} size="sm">
            Small
          </Button>
          <Button variant={variant}>Default</Button>
          <Button variant={variant} size="lg">
            Large
          </Button>
          <Button variant={variant}>
            <Play aria-hidden="true" />
            With icon
          </Button>
          <Button variant={variant} size="icon" aria-label={`${variant} icon`}>
            <Plus aria-hidden="true" />
          </Button>
          <Button
            variant={variant}
            size="icon-sm"
            aria-label={`${variant} small icon`}
          >
            <Plus aria-hidden="true" />
          </Button>
          <Button variant={variant} disabled>
            Disabled
          </Button>
        </GalleryRow>
      ))}

      <GalleryRow label="invalid, and buttonVariants on a link">
        <Button variant="outline" aria-invalid="true">
          Invalid
        </Button>
        <a
          href="#gallery-buttons"
          className={buttonVariants({ variant: "outline", size: "sm" })}
        >
          Link as button
        </a>
      </GalleryRow>

      {/* The overrides production call sites pass today. Each becomes a
          variant; this row must not move a pixel when it does. */}
      <GalleryRow label="call-site recipes">
        <Button
          variant="ghost"
          size="icon"
          aria-label="Muted ghost"
          className="relative text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <Bell aria-hidden="true" />
        </Button>
        <Button
          variant="outline"
          className="border-border bg-muted text-muted-foreground hover:bg-accent hover:text-foreground"
        >
          Dialog secondary
        </Button>
        <Button
          variant="outline"
          className="border-border bg-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          Dialog cancel
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="border-destructive/50 text-destructive hover:bg-destructive/10 hover:text-destructive"
        >
          <Trash2 aria-hidden="true" />
          Outline destructive
        </Button>
        <Button variant="outline" className={SETTINGS_RESET_BUTTON_CLASS}>
          <RotateCcw aria-hidden="true" />
          Reset
        </Button>
        <Button
          variant="accent"
          className="font-semibold shadow-glow"
        >
          <Play aria-hidden="true" />
          Play all
        </Button>
        <Button variant="outline" className="rounded-full font-semibold">
          <Shuffle aria-hidden="true" />
          Shuffle
        </Button>
        <Button variant="outline" className="min-h-10 rounded-full">
          <Heart aria-hidden="true" />
          Liked
        </Button>
      </GalleryRow>
    </GallerySection>
  );
}

function BadgesSection() {
  return (
    <GallerySection id="badges" title="Badge and Spinner">
      <GalleryRow label="badge variants and call-site recipes">
        <Badge>Owner</Badge>
        <Badge variant="outline">Outline</Badge>
        <Badge variant="outline" className="px-3 py-1">
          12 movies
        </Badge>
        <Badge variant="outline" className="px-3 py-1 text-sm font-normal">
          Pop, Rock
        </Badge>
        <span
          className={cn(
            "inline-flex rounded-lg p-3",
            MEDIA_FALLBACK_SURFACE_CLASS,
          )}
        >
          <Badge variant="outline" className={OVER_MEDIA_BADGE_CLASS}>
            PG-13
          </Badge>
        </span>
      </GalleryRow>

      <GalleryRow label="spinner sizes and tone">
        <Spinner />
        <Spinner className="size-5" />
        <Spinner className="size-6 text-primary" />
        <Spinner className="size-8 text-primary" />
        <Spinner className="size-10 text-primary" />
      </GalleryRow>
    </GallerySection>
  );
}

function FieldsSection() {
  return (
    <GallerySection id="fields" title="Input, Label, Checkbox, Switch">
      <div className="grid gap-4 sm:grid-cols-2">
        <div>
          <Label htmlFor="gallery-input-default">Default</Label>
          <Input
            id="gallery-input-default"
            placeholder="Placeholder"
            variant="outline"
          />
        </div>
        <div>
          <Label htmlFor="gallery-input-value">With a value</Label>
          <Input
            id="gallery-input-value"
            defaultValue="Casino Royale"
            variant="outline"
            readOnly
          />
        </div>
        <div>
          <Label htmlFor="gallery-input-disabled">Disabled</Label>
          <Input
            id="gallery-input-disabled"
            placeholder="Disabled"
            variant="outline"
            disabled
          />
        </div>
        <div>
          <Label htmlFor="gallery-input-invalid">Invalid</Label>
          <Input
            id="gallery-input-invalid"
            defaultValue="not-an-email"
            variant="outline"
            aria-invalid="true"
            readOnly
          />
        </div>
        <div>
          <Label htmlFor="gallery-input-muted" className="text-foreground">
            Recipe: muted field
          </Label>
          <Input id="gallery-input-muted" placeholder="Playlist name" />
        </div>
        <div>
          <Label
            htmlFor="gallery-input-settings"
            className="text-muted-foreground"
          >
            Recipe: settings field
          </Label>
          <Input
            id="gallery-input-settings"
            placeholder="/media/static"
            variant="inset"
            size="lg"
          />
        </div>
        <div>
          <Label htmlFor="gallery-input-glass">Recipe: glass field</Label>
          <Input
            id="gallery-input-glass"
            placeholder="Search…"
            variant="glass"
          />
        </div>
        <div>
          <Label htmlFor="gallery-select-default">Select</Label>
          <Select value="recommended" onValueChange={noop}>
            <SelectTrigger
              id="gallery-select-default"
              variant="outline"
              className="w-fit"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="recommended">Use recommended</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <Label htmlFor="gallery-select-settings">
            Recipe: settings select
          </Label>
          <Select value="two" onValueChange={noop}>
            <SelectTrigger
              id="gallery-select-settings"
              variant="inset"
              size="lg"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="two">2 trailers</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <Label htmlFor="gallery-select-playback">
            Recipe: playback select
          </Label>
          <Select value="1080" onValueChange={noop}>
            <SelectTrigger id="gallery-select-playback">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="1080">1080p · 8 Mbps</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <GalleryRow label="checkbox">
        <Checkbox aria-label="Unchecked" />
        <Checkbox aria-label="Checked" checked onCheckedChange={noop} />
        <Checkbox aria-label="Disabled" disabled />
        <Checkbox
          aria-label="Disabled and checked"
          checked
          disabled
          onCheckedChange={noop}
        />
      </GalleryRow>

      <div className="grid gap-4 sm:grid-cols-2">
        <SwitchField
          id="gallery-switch-off"
          label="Library watcher"
          description="Apply added, changed, and removed library files as they happen."
          checked={false}
          onCheckedChange={noop}
          icon={<Activity className="size-5 text-success" aria-hidden="true" />}
        />
        <SwitchField
          id="gallery-switch-on"
          label="Download images"
          description="Save library artwork in the static directory and serve it from there."
          checked
          onCheckedChange={noop}
          icon={<Film className="size-5 text-primary" aria-hidden="true" />}
        />
      </div>
    </GallerySection>
  );
}

function SurfacesSection() {
  return (
    <GallerySection id="surfaces" title="Card, Alert, Avatar, Separator">
      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Card title</CardTitle>
            <CardDescription>Default card surface.</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">Card content.</p>
          </CardContent>
          <CardFooter>
            <Button variant="outline" size="sm">
              Action
            </Button>
          </CardFooter>
        </Card>

        <Card className={SETTINGS_CARD_SURFACE_CLASS}>
          <CardHeader>
            <CardTitle asChild>
              <h3 className="flex items-center gap-2 text-foreground">
                <ListMusic className="size-5 text-primary" aria-hidden="true" />
                Recipe: settings card
              </h3>
            </CardTitle>
            <CardDescription>
              The surface every Settings card passes today.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Separator className="bg-accent/50" />
          </CardContent>
        </Card>
      </div>

      {/* Not a live alert: the gallery must not announce itself. */}
      <Alert role="presentation">
        <AlertCircle aria-hidden="true" />
        <AlertTitle>Couldn’t load movies.</AlertTitle>
        <AlertDescription>Default alert.</AlertDescription>
      </Alert>
      <Alert
        role="presentation"
        className="border-destructive/25 bg-destructive/10 text-destructive"
      >
        <AlertCircle className="size-4" aria-hidden="true" />
        <AlertDescription>Recipe: section error alert.</AlertDescription>
      </Alert>

      <GalleryRow label="avatar">
        <Avatar>
          <AvatarFallback>AD</AvatarFallback>
        </Avatar>
        <Avatar className="size-10 shrink-0">
          <AvatarFallback className="bg-accent text-xs font-semibold text-foreground">
            RL
          </AvatarFallback>
        </Avatar>
        <Avatar className="size-20">
          <AvatarFallback className="bg-primary/20 text-lg text-primary">
            JI
          </AvatarFallback>
        </Avatar>
      </GalleryRow>

      <Separator />
    </GallerySection>
  );
}

function NavigationSection() {
  return (
    <GallerySection id="navigation" title="Tabs and Pagination">
      <Tabs value="all" onValueChange={noop}>
        <TabsList>
          <TabsTrigger value="all">All</TabsTrigger>
          <TabsTrigger value="genres">Genres</TabsTrigger>
          <TabsTrigger value="playlists" disabled>
            Disabled
          </TabsTrigger>
        </TabsList>
        <TabsContent value="all">
          <p className="text-sm text-muted-foreground">Default tabs panel.</p>
        </TabsContent>
      </Tabs>

      <Tabs value="genres" onValueChange={noop}>
        <TabsList
          className={cn(LIBRARY_TABS_LIST_CLASS, "grid-cols-3 sm:grid-cols-3")}
        >
          <TabsTrigger value="all" className={LIBRARY_TAB_TRIGGER_CLASS}>
            All Movies
          </TabsTrigger>
          <TabsTrigger value="genres" className={LIBRARY_TAB_TRIGGER_CLASS}>
            Genres
          </TabsTrigger>
          <TabsTrigger value="playlists" className={LIBRARY_TAB_TRIGGER_CLASS}>
            Playlists
          </TabsTrigger>
        </TabsList>
        <TabsContent value="genres">
          <p className="text-sm text-muted-foreground">
            Recipe: library tabs.
          </p>
        </TabsContent>
      </Tabs>

      <LibraryPagination currentPage={2} totalPages={9} onPageChange={noop} />
    </GallerySection>
  );
}

// One sample per entry of the scales in src/lib/theme-tokens.ts. `data-token`
// names the utility, so the visual spec can read back what it computes to.
function TokensSection() {
  return (
    <GallerySection id="tokens" title="Elevation, layers, durations">
      <GalleryRow label="elevation">
        <div
          data-token="shadow-floating"
          className="rounded-xl border border-border bg-card px-4 py-3 text-sm text-foreground shadow-floating"
        >
          shadow-floating
        </div>
        <div
          data-token="shadow-glow"
          className="rounded-md bg-primary px-4 py-3 text-sm font-medium text-primary-foreground shadow-glow"
        >
          shadow-glow
        </div>
      </GalleryRow>

      <GalleryRow label="layers">
        <span
          data-token="z-chrome"
          className="z-chrome rounded-md border border-border px-3 py-1 text-sm text-foreground"
        >
          z-chrome
        </span>
        <span
          data-token="z-modal"
          className="z-modal rounded-md border border-border px-3 py-1 text-sm text-foreground"
        >
          z-modal
        </span>
        <span
          data-token="z-popover"
          className="z-popover rounded-md border border-border px-3 py-1 text-sm text-foreground"
        >
          z-popover
        </span>
      </GalleryRow>

      <GalleryRow label="durations">
        <span
          data-token="duration-micro"
          className="rounded-md border border-border px-3 py-1 text-sm text-foreground transition-opacity duration-micro motion-reduce:transition-none"
        >
          duration-micro
        </span>
        <span
          data-token="duration-standard"
          className="rounded-md border border-border px-3 py-1 text-sm text-foreground transition-opacity duration-standard motion-reduce:transition-none"
        >
          duration-standard
        </span>
        <span
          data-token="duration-page"
          className="rounded-md border border-border px-3 py-1 text-sm text-foreground transition-opacity duration-page motion-reduce:transition-none"
        >
          duration-page
        </span>
      </GalleryRow>
    </GallerySection>
  );
}

// In page order. The id is each section's `data-gallery` value.
const SECTIONS = [
  ["buttons", ButtonsSection],
  ["badges", BadgesSection],
  ["fields", FieldsSection],
  ["surfaces", SurfacesSection],
  ["navigation", NavigationSection],
  ["tokens", TokensSection],
] as const;

type GalleryPrimitivesProps = {
  /** Render this section alone; an unknown id renders none. */
  only?: string;
};

export default function GalleryPrimitives({ only }: GalleryPrimitivesProps) {
  return (
    <>
      {SECTIONS.filter(([id]) => only === undefined || id === only).map(
        ([id, Section]) => (
          <Section key={id} />
        ),
      )}
    </>
  );
}
