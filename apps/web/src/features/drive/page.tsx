import { ArrowUpRight, FolderPlus, HardDrive, Users } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardHeader,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";
import { useSession } from "@/features/administration/auth";
import {
  ErrorBanner,
  Loading,
  PageHeading,
} from "@/features/administration/shared";
import {
  drivePersonalEnsure,
  driveSpaceCreate,
  driveSpaces,
} from "@/lib/capabilities";
import { NameDialog } from "./name-dialog";

export function DrivePage() {
  const { t } = useTranslation(["drive", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const [creating, setCreating] = useState(false);
  const canUse = session.permissions.includes("drive:use");
  const identity = `${session.tenantId}:${session.account.id}:${session.activeDepartmentId}`;
  const key = ["drive", identity, "spaces"];
  const ensured = useRef("");
  const { mutate: ensure, error: ensureError } = useMutation({
    mutationFn: drivePersonalEnsure,
    onSuccess: () => client.invalidateQueries({ queryKey: key }),
  });
  useEffect(() => {
    if (canUse && ensured.current !== identity) {
      ensured.current = identity;
      ensure();
    }
  }, [canUse, ensure, identity]);
  const query = useQuery({
    queryKey: key,
    queryFn: driveSpaces,
    enabled: canUse,
    retry: false,
  });
  if (!canUse)
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading title={t("drive:heading")} subtitle={t("drive:description")}>
        {session.permissions.includes("drive:create") && (
          <Button type="button" onClick={() => setCreating(true)}>
            <FolderPlus />
            {t("drive:createSpace")}
          </Button>
        )}
      </PageHeading>
      {ensureError && <ErrorBanner error={ensureError} />}
      {query.isError && (
        <ErrorBanner
          error={query.error}
          onRetry={() => {
            void query.refetch();
          }}
        />
      )}
      {query.data?.data.items.length === 0 && (
        <div className="admin-empty">{t("drive:noSpaces")}</div>
      )}
      <div className="role-card-grid">
        {query.data?.data.items.map((space) => (
          <Card key={space.id} className="role-card">
            <CardHeader>
              <div className="flex items-center justify-between gap-4">
                <span className="flex size-10 items-center justify-center rounded-lg bg-muted">
                  {space.kind === "personal" ? (
                    <HardDrive aria-hidden="true" size={20} />
                  ) : (
                    <Users aria-hidden="true" size={20} />
                  )}
                </span>
                <Badge variant="outline">
                  {t(`drive:roles.${space.role}`)}
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              <h2 className="truncate">
                <Link
                  className="hover:underline focus-visible:underline"
                  to={`/drive/${space.id}`}
                >
                  {space.kind === "personal" ? t("drive:personal") : space.name}
                </Link>
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                {t(
                  space.kind === "personal" ? "drive:personal" : "drive:shared",
                )}
              </p>
            </CardContent>
            <CardFooter>
              <Button
                variant="ghost"
                render={<Link to={`/drive/${space.id}`} />}
              >
                {t("drive:open")}
                <ArrowUpRight aria-hidden="true" />
              </Button>
            </CardFooter>
          </Card>
        ))}
      </div>
      {creating && (
        <NameDialog
          title={t("drive:createSpace")}
          maxLength={120}
          onClose={() => setCreating(false)}
          onSave={async (name) => {
            await driveSpaceCreate(name);
            await client.invalidateQueries({ queryKey: key });
          }}
        />
      )}
    </>
  );
}
