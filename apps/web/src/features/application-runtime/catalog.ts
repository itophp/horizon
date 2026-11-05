import { useQuery } from "@tanstack/react-query";
import { runtimeRead } from "@/lib/capabilities";
import { runtimeApplicationV2Schema } from "@/api/generated/runtime";
import { useSession } from "@/features/administration/auth";
import { z } from "zod";
export function useApplicationCatalog() {
  const session = useSession();
  return useQuery({
    queryKey: [
      "runtime-catalog",
      session.tenantId,
      session.account.id,
      session.activeDepartmentId,
      session.authorizationVersion,
    ],
    queryFn: async ({ signal }) =>
      z
        .object({ items: z.array(runtimeApplicationV2Schema) })
        .parse(await runtimeRead("application.runtime.list", {}, signal)),
    retry: false,
    refetchInterval: 15000,
  });
}
