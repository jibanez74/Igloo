import type { components } from "./openapi.gen";

type Schema = components["schemas"];

export type SettingsType = Schema["SettingsData"];
export type UpdateLibrarySettingsRequest = Schema["UpdateLibrarySettingsRequest"];
export type UpdateLibrarySettingsResponseType = Schema["UpdateLibrarySettingsData"];
export type HardwareAccelerationDevice = Schema["HardwareAccelerationDevice"];
export type GeneralSettingsType = Schema["GeneralSettings"];
export type UpdateGeneralSettingsRequest = Schema["UpdateGeneralSettingsRequest"];
export type GeneralSettingsResponseType = Schema["GeneralSettingsData"];
export type UpdateGeneralSettingsResponseType = Schema["UpdateGeneralSettingsData"];
export type PlaybackProfileType = Schema["PlaybackProfile"];
export type PlaybackSettingsType = Schema["PlaybackSettings"];
export type UpdatePlaybackSettingsRequest = Schema["UpdatePlaybackSettingsRequest"];
export type PlaybackSettingsResponseType = Schema["PlaybackSettingsData"];
