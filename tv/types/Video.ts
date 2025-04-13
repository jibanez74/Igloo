export type Video = {
  ID: number;
  title: string;
  index: number;
  profile: string;
  aspectRatio: string;
  bitDepth: string;
  codec: "aac" | "mp3" | "copy";
  width: number;
  height: number;
  codedHeight: number;
  codedWidth: number;
  colorTransfer: string;
  colorPrimaries: string;
  colorSpace: string;
  frameRate: string;
  avgFrameRate: string;
  CreatedAt?: string | Date;
  UpdatedAt?: string | Date;
  DeletedAt?: string | Date | null;
};

export type VideoError = {
  error: {
    errorString?: string;
    errorException?: string;
  };
};
