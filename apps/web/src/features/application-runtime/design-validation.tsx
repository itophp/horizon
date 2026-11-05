import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { runtimeDefinitionValidationSchema } from "@/api/generated/runtime";
import { runtimeRead } from "@/lib/capabilities";
import { useSession } from "@/features/administration/auth";
import { ErrorBanner } from "@/features/administration/shared";

export function DesignValidation({
  applicationId,
  version,
  revision,
  identity,
}: {
  applicationId: string;
  version: string;
  revision: number;
  identity: string;
}) {
  const session = useSession();
  const { t } = useTranslation("runtime");
  const query = useQuery({
    queryKey: [
      "runtime-design-validation",
      identity,
      applicationId,
      version,
      revision,
    ],
    enabled: session.permissions.includes("applications:design"),
    queryFn: async ({ signal }) =>
      runtimeDefinitionValidationSchema.parse(
        await runtimeRead(
          "application.runtime.definition.validate",
          { applicationId, version },
          signal,
        ),
      ),
    retry: false,
  });
  if (query.isError) return <ErrorBanner error={query.error} />;
  if (!query.data) return null;
  return query.data.valid ? (
    <p role="status">{t("design.validated")}</p>
  ) : (
    <ErrorBanner error={new Error(query.data.code)} />
  );
}
