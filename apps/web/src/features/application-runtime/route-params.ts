import { createContext } from "react";
export const RuntimeRouteParams = createContext<
  Record<string, string | undefined>
>({});
