// DEVICE TYPES
// Types for paired TV / mobile devices (Quick Connect + device tokens)

export type DeviceType = {
  id: number;
  name: string;
  platform: string;
  app_version: string | null;
  created_at: string;
  last_used_at: string;
  is_current: boolean;
};

export type DevicesListResponseType = {
  devices: DeviceType[];
};
