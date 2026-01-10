import { createFileRoute } from "@tanstack/react-router";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export const Route = createFileRoute("/_auth/music/")({
  component: MusicPage,
});

function MusicPage() {
  return (
    <div>
      {/* Page header */}
      <header className='mb-8'>
        <h1 className='text-3xl md:text-4xl font-semibold tracking-tight text-white flex items-center gap-3'>
          <i
            className='fa-solid fa-music text-amber-400 text-2xl'
            aria-hidden='true'
          />
          <span>Music Library</span>
        </h1>
        <p className='mt-2 text-slate-400 max-w-2xl text-sm md:text-base'>
          Browse your collection of musicians, albums, and tracks
        </p>
      </header>

      {/* Tabs */}
      <Tabs defaultValue='albums'>
        <TabsList className='bg-slate-800/50 border border-slate-700/50 h-auto p-1'>
          <TabsTrigger
            value='musicians'
            className='data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 text-slate-400 hover:text-white px-4 py-2'
          >
            <i className='fa-solid fa-user-group mr-2' aria-hidden='true' />
            Musicians
          </TabsTrigger>
          <TabsTrigger
            value='albums'
            className='data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 text-slate-400 hover:text-white px-4 py-2'
          >
            <i className='fa-solid fa-compact-disc mr-2' aria-hidden='true' />
            Albums
          </TabsTrigger>
          <TabsTrigger
            value='tracks'
            className='data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 text-slate-400 hover:text-white px-4 py-2'
          >
            <i className='fa-solid fa-list mr-2' aria-hidden='true' />
            Tracks
          </TabsTrigger>
        </TabsList>

        <TabsContent value='musicians' className='mt-6'>
          <div className='text-slate-400'>Musicians content coming soon...</div>
        </TabsContent>

        <TabsContent value='albums' className='mt-6'>
          <div className='text-slate-400'>Albums content coming soon...</div>
        </TabsContent>

        <TabsContent value='tracks' className='mt-6'>
          <div className='text-slate-400'>Tracks content coming soon...</div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
