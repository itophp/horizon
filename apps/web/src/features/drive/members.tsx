import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useSession } from "@/features/administration/auth";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  Loading,
  PageHeading,
  type AdminColumn,
} from "@/features/administration/shared";
import {
  driveMemberAdd,
  driveMemberChange,
  driveMemberRemove,
  driveMembers,
  driveSpaces,
  type DriveMember,
} from "@/lib/capabilities";

const memberSchema = z.object({
  targetType: z.enum(["account", "department"]),
  targetId: z.number().int().positive(),
  role: z.enum(["admin", "editor", "viewer"]),
});
type Values = z.infer<typeof memberSchema>;

export function DriveMembersPage() {
  const { t } = useTranslation(["drive", "admin"]);
  const session = useSession();
  const client = useQueryClient();
  const spaceId = Number(useParams().id);
  const identity = `${session.tenantId}:${session.account.id}:${session.activeDepartmentId}`;
  const key = ["drive", identity, spaceId, "members"];
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<DriveMember>();
  const spaces = useQuery({
    queryKey: ["drive", identity, "spaces"],
    queryFn: driveSpaces,
    enabled: session.permissions.includes("drive:use") && spaceId > 0,
    retry: false,
  });
  const canManage =
    spaces.data?.data.items.some(
      (space) =>
        space.id === spaceId &&
        space.kind === "shared" &&
        space.role === "admin",
    ) ?? false;
  const query = useQuery({
    queryKey: key,
    queryFn: () => driveMembers(spaceId),
    enabled: canManage,
    retry: false,
  });
  const change = useMutation({
    mutationFn: ({ id, role }: { id: number; role: DriveMember["role"] }) =>
      driveMemberChange(spaceId, id, role),
    onSuccess: () =>
      client.invalidateQueries({ queryKey: ["drive", identity] }),
  });
  const remove = useMutation({
    mutationFn: (id: number) => driveMemberRemove(spaceId, id),
    onSuccess: async () => {
      setRemoving(undefined);
      await client.invalidateQueries({ queryKey: ["drive", identity] });
    },
  });
  const columns: AdminColumn<DriveMember>[] = [
    {
      id: "type",
      header: t("drive:targetType"),
      cell: (row) =>
        t(row.targetType === "account" ? "drive:account" : "drive:department"),
    },
    { id: "target", header: t("drive:targetId"), cell: (row) => row.targetId },
    {
      id: "role",
      header: t("drive:role"),
      cell: (row) => (
        <select
          className="toolbar-select"
          aria-label={t("drive:changeRole")}
          value={row.role}
          disabled={change.isPending}
          onChange={(event) =>
            change.mutate({
              id: row.id,
              role: event.target.value as DriveMember["role"],
            })
          }
        >
          <option value="admin">{t("drive:roles.admin")}</option>
          <option value="editor">{t("drive:roles.editor")}</option>
          <option value="viewer">{t("drive:roles.viewer")}</option>
        </select>
      ),
    },
    {
      id: "actions",
      header: t("drive:actions"),
      cell: (row) => (
        <Button type="button" variant="ghost" onClick={() => setRemoving(row)}>
          {t("drive:remove")}
        </Button>
      ),
    },
  ];
  if (
    !session.permissions.includes("drive:use") ||
    !Number.isSafeInteger(spaceId) ||
    spaceId < 1
  )
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (spaces.isPending) return <Loading />;
  if (spaces.isError) return <ErrorBanner error={spaces.error} />;
  if (!canManage)
    return <div className="admin-empty">{t("admin:errors.forbidden")}</div>;
  if (query.isPending) return <Loading />;
  return (
    <>
      <PageHeading
        title={t("drive:members")}
        subtitle={t("drive:editDescription")}
      >
        <Link to={`/drive/${spaceId}`}>{t("drive:back")}</Link>
        <Button onClick={() => setAdding(true)}>{t("drive:addMember")}</Button>
      </PageHeading>
      {query.isError && <ErrorBanner error={query.error} />}
      {change.isError && <ErrorBanner error={change.error} />}
      <DataTable rows={query.data?.data.items ?? []} columns={columns} />
      {adding && (
        <MemberForm
          spaceId={spaceId}
          onClose={() => setAdding(false)}
          onSaved={() =>
            client.invalidateQueries({ queryKey: ["drive", identity] })
          }
        />
      )}
      {removing && (
        <EditorDialog
          open
          onOpenChange={(open) => {
            if (!open) setRemoving(undefined);
          }}
          title={t("drive:remove")}
          description={t("drive:editDescription")}
        >
          {remove.isError && <ErrorBanner error={remove.error} />}
          <div className="editor-footer">
            <Button
              variant="outline"
              type="button"
              onClick={() => setRemoving(undefined)}
            >
              {t("drive:cancel")}
            </Button>
            <Button
              variant="destructive"
              type="button"
              disabled={remove.isPending}
              onClick={() => remove.mutate(removing.id)}
            >
              {t("drive:remove")}
            </Button>
          </div>
        </EditorDialog>
      )}
    </>
  );
}

function MemberForm({
  spaceId,
  onClose,
  onSaved,
}: {
  spaceId: number;
  onClose: () => void;
  onSaved: () => Promise<unknown>;
}) {
  const { t } = useTranslation("drive");
  const form = useForm<Values>({
    resolver: zodResolver(memberSchema),
    defaultValues: { targetType: "account", targetId: 0, role: "viewer" },
  });
  const add = useMutation({
    mutationFn: (value: Values) =>
      driveMemberAdd(spaceId, value.targetType, value.targetId, value.role),
    onSuccess: async () => {
      await onSaved();
      onClose();
    },
  });
  return (
    <EditorDialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t("addMember")}
      description={t("editDescription")}
    >
      <form onSubmit={form.handleSubmit((value) => add.mutate(value))}>
        <label htmlFor="drive-member-type">{t("targetType")}</label>
        <select
          id="drive-member-type"
          className="toolbar-select"
          {...form.register("targetType")}
        >
          <option value="account">{t("account")}</option>
          <option value="department">{t("department")}</option>
        </select>
        <label htmlFor="drive-member-id">{t("targetId")}</label>
        <Input
          id="drive-member-id"
          type="number"
          min={1}
          {...form.register("targetId", { valueAsNumber: true })}
        />
        {form.formState.errors.targetId && <p role="alert">{t("required")}</p>}
        <label htmlFor="drive-member-role">{t("role")}</label>
        <select
          id="drive-member-role"
          className="toolbar-select"
          {...form.register("role")}
        >
          <option value="admin">{t("roles.admin")}</option>
          <option value="editor">{t("roles.editor")}</option>
          <option value="viewer">{t("roles.viewer")}</option>
        </select>
        {add.isError && <ErrorBanner error={add.error} />}
        <div className="editor-footer">
          <Button type="button" variant="outline" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button type="submit" disabled={add.isPending}>
            {t("save")}
          </Button>
        </div>
      </form>
    </EditorDialog>
  );
}
