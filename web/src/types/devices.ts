// DEVICE TYPES
// Types for paired TV / mobile devices (Quick Connect + device tokens)

export type DeviceType = {
  id: number;
  name: string;
  platform: string;
  app_version: string | null;
  created_at: string;
  last_used_at: string;
  // Only meaningful in device-login/redeem responses; the session-only
  // devices list always reports false.
  is_current: boolean;
};

export type DevicesListResponseType = {
  devices: DeviceType[];
};

// Pending device behind a quick-connect code, before it is approved.
export type QuickConnectLookupType = {
  device_name: string;
  platform: string;
  app_version: string | null;
};
