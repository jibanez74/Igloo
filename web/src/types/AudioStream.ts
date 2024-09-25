export type AudioStream = {
  ID?: number;
  title: string;
  index: number;
  profile: string;
  codec: string;
  channels: number;
  channelLayout: string;
  language: string;
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};
