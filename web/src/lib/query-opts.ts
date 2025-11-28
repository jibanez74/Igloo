import { queryOptions } from "@tanstack/react-query";
import { getAuthUser } from "@/lib/api";
import { AUTH_USER_KEY } from "@/lib/constants";

export function authUserQueryOpts() {
  return queryOptions({
    queryKey: [AUTH_USER_KEY],
    queryFn: getAuthUser,
  });
}
