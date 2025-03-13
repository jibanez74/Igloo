export enum TmdbJob {
  Director = "Director",
  Producer = "Producer",
  ExecutiveProducer = "Executive Producer",
  Screenplay = "Screenplay",
  Writer = "Writer",
  DirectorOfPhotography = "Director of Photography",
  Editor = "Editor",
  ProductionDesigner = "Production Designer",
  ArtDirector = "Art Director",
  CostumeDesigner = "Costume Designer",
  MakeupArtist = "Makeup Artist",
  OriginalMusicComposer = "Original Music Composer",
  SoundDesigner = "Sound Designer",
}

export type Crew = {
  id: number;
  name: string;
  thumb: string;
  job: TmdbJob | string;
  sort_order: number;
};

export const jobRank: Record<TmdbJob | string, number> = {
  [TmdbJob.Director]: 1,
  [TmdbJob.Producer]: 2,
  [TmdbJob.ExecutiveProducer]: 3,
  [TmdbJob.Screenplay]: 4,
  [TmdbJob.Writer]: 5,
  [TmdbJob.DirectorOfPhotography]: 6,
  [TmdbJob.Editor]: 7,
  [TmdbJob.ProductionDesigner]: 8,
  [TmdbJob.ArtDirector]: 9,
  [TmdbJob.CostumeDesigner]: 10,
  [TmdbJob.MakeupArtist]: 11,
  [TmdbJob.OriginalMusicComposer]: 12,
  [TmdbJob.SoundDesigner]: 13,
};