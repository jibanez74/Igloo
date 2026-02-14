import { createLazyFileRoute } from "@tanstack/react-router";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Play } from "lucide-react";

export const Route = createLazyFileRoute("/_auth/settings/playback")({
  component: PlaybackSettings,
});

function PlaybackSettings() {
  return (
    <div className="space-y-8">
      <Card className="border-slate-700/50 bg-slate-800/30">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-white">
            <Play className="size-5 text-amber-400" aria-hidden="true" />
            Playback Settings
          </CardTitle>
          <CardDescription className="text-slate-400">
            Configure audio and video playback preferences
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-slate-400">Playback settings coming soon...</p>
        </CardContent>
      </Card>
    </div>
  );
}
