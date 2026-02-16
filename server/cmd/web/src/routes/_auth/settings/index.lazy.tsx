import { createLazyFileRoute } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Sliders } from "lucide-react";

export const Route = createLazyFileRoute("/_auth/settings/")({
  component: GeneralSettings,
});

function GeneralSettings() {
  return (
    <div className='space-y-8'>
      <Card className='border-slate-700/50 bg-slate-800/30'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-white'>
            <Sliders className='size-5 text-amber-400' aria-hidden='true' />
            General Settings
          </CardTitle>
          <CardDescription className='text-slate-300'>
            Configure general application settings
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p className='text-slate-300'>General settings coming soon...</p>
        </CardContent>
      </Card>
    </div>
  );
}
