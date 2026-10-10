import type { components } from "./openapi.gen";

export type AuthUser = components["schemas"]["AuthUser"];
export type AdminUserType = components["schemas"]["AdminUser"];
/** A user as other users see them: id, name, avatar. */
export type UserSummaryType = components["schemas"]["UserSummary"];
