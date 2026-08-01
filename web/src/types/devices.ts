import type { components } from "./openapi.gen";

type Schema = components["schemas"];

export type DeviceType = Schema["Device"];
export type DevicesListResponseType = Schema["DevicesListData"];
export type QuickConnectLookupType = Schema["QuickConnectLookupData"];
