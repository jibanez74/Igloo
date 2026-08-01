import type { components } from "./openapi.gen";

type Schema = components["schemas"];

export type SettingsType = Schema["SettingsData"];
export type UpdateLibrarySettingsRequest = Schema["UpdateLibrarySettingsRequest"];
export type UpdateLibrarySettingsResponseType = Schema["UpdateLibrarySettingsEnvelope"]["data"];
export type HardwareAccelerationDevice = Schema["HardwareAccelerationDevice"];
export type GeneralSettingsType = Schema["GeneralSettings"];
export type UpdateGeneralSettingsRequest = Schema["UpdateGeneralSettingsRequest"];
export type GeneralSettingsResponseType = Schema["GeneralSettingsEnvelope"]["data"];
export type UpdateGeneralSettingsResponseType = Schema["UpdateGeneralSettingsEnvelope"]["data"];
export type PlaybackProfileType = Schema["PlaybackProfile"];
export type PlaybackSettingsType = Schema["PlaybackSettings"];
export type UpdatePlaybackSettingsRequest = Schema["UpdatePlaybackSettingsRequest"];
export type PlaybackSettingsResponseType = Schema["PlaybackSettingsEnvelope"]["data"];
export type UpdatePlaybackSettingsResponseType = Schema["UpdatePlaybackSettingsEnvelope"]["data"];
