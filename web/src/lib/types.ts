export type SimpleMusician = {
  id: number;
  name: string;
};

export type SimpleAlbum = {
  id: number;
  title: string;
  cover?: string;
  musician_name?: string;
};

export type Track = {
  id: number;
  title: string;
  src: string;
  duration: number;
  musician_name?: string;
  cover?: string; // album/track cover
};
