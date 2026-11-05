import { z } from "zod";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { runtimeRead, runtimeExecute } from "@/lib/capabilities";
import { runtimeInstallationSchema } from "@/api/generated/runtime";
import { useSession } from "@/features/administration/auth";
import { Button } from "@/components/ui/button";
import { ErrorBanner } from "@/features/administration/shared";
export function DesignActivation({
  applicationId,
  source,
  enabled,
  identity,
}: {
  applicationId: string;
  source: string;
  enabled: boolean;
  identity: string;
}) {
  const session = useSession();
  const { t } = useTranslation("runtime");
  const client = useQueryClient();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<unknown>();
  const installation = useQuery({
    queryKey: ["runtime-design-installation", identity, source],
    queryFn: async ({ signal }) =>
      z
        .object({ installation: runtimeInstallationSchema })
        .parse(
          await runtimeRead(
            "application.runtime.installation.get",
            { pluginId: source },
            signal,
          ),
        ),
    retry: false,
  });
  return (
    <>
      {installation.isError && <ErrorBanner error={installation.error} />}{" "}
      {error !== undefined && <ErrorBanner error={error} />}
      <Button
        type="button"
        disabled={
          pending ||
          !installation.data ||
          !session.permissions.includes("applications:manage")
        }
        onClick={async () => {
          if (!installation.data) return;
          setPending(true);
          setError(undefined);
          try {
            const result = await runtimeExecute(
              `application.runtime.application.${enabled ? "disable" : "enable"}`,
              "1.0.0",
              {
                pluginId: source,
                applicationId,
                revision: installation.data.installation.revision,
              },
              crypto.randomUUID(),
              crypto.randomUUID(),
            );
            if (result.status !== "succeeded")
              throw new Error(result.errorCode || "unavailable");
            await client.invalidateQueries({
              predicate: (query) =>
                typeof query.queryKey[0] === "string" &&
                query.queryKey[0].startsWith("runtime"),
            });
          } catch (error) {
            setError(error);
          } finally {
            setPending(false);
          }
        }}
      >
        {t(enabled ? "design.disable" : "design.enable")}
      </Button>
    </>
  );
}
