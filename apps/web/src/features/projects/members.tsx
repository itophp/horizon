import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { EditorDialog } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  projectMemberAdd,
  projectMemberRemove,
  projectMemberSearch,
  projectMembers,
  projectOwnerTransfer,
  type ProjectAccount,
} from "@/lib/capabilities";

/**
 * MembersPanel is the only place the visibility boundary is edited. Adding a
 * member happens in a dialog so the tab stays a list, not a form.
 */
export function MembersPanel({
  projectId,
  isOwner,
  currentAccountId,
  archived,
}: {
  projectId: number;
  isOwner: boolean;
  currentAccountId: number;
  archived: boolean;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const [adding, setAdding] = useState(false);

  const members = useQuery({
    queryKey: ["project-members", projectId],
    queryFn: () => projectMembers(projectId),
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: async (action: () => Promise<unknown>) => action(),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-members", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      await client.invalidateQueries({ queryKey: ["projects"] });
    },
  });

  const editable = isOwner && !archived;
  const rows = members.data?.data.items ?? [];

  return (
    <section className="space-y-3">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold">{t("members.heading")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("members.description")}
          </p>
        </div>
        {editable && (
          <Button onClick={() => setAdding(true)}>{t("members.add")}</Button>
        )}
      </div>
      {(members.isError || mutation.isError) && (
        <p role="alert">{t("error")}</p>
      )}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("name")}</TableHead>
            <TableHead>{t("members.email")}</TableHead>
            <TableHead>{t("members.role")}</TableHead>
            <TableHead>{t("actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.accountId}>
              <TableCell>
                {row.name}
                {row.accountId === currentAccountId ? ` (${t("you")})` : ""}
              </TableCell>
              <TableCell>{row.email || t("dash")}</TableCell>
              <TableCell>
                {row.isOwner
                  ? t("members.ownerBadge")
                  : t("members.memberBadge")}
              </TableCell>
              <TableCell>
                {editable && !row.isOwner && (
                  <div className="flex gap-2">
                    <Button
                      variant="outline"
                      disabled={mutation.isPending}
                      onClick={() =>
                        mutation.mutate(async () => {
                          await projectOwnerTransfer(projectId, row.accountId);
                        })
                      }
                    >
                      {t("members.makeOwner")}
                    </Button>
                    <Button
                      variant="outline"
                      disabled={mutation.isPending}
                      onClick={() =>
                        mutation.mutate(async () => {
                          await projectMemberRemove(projectId, row.accountId);
                        })
                      }
                    >
                      {t("members.remove")}
                    </Button>
                  </div>
                )}
              </TableCell>
            </TableRow>
          ))}
          {rows.length === 0 && (
            <TableRow>
              <TableCell colSpan={4}>{t("empty")}</TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      {editable && (
        <AddMemberDialog
          open={adding}
          onOpenChange={setAdding}
          projectId={projectId}
        />
      )}
    </section>
  );
}

function AddMemberDialog({
  open,
  onOpenChange,
  projectId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  projectId: number;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const [keyword, setKeyword] = useState("");
  const [results, setResults] = useState<ProjectAccount[]>([]);
  const [searched, setSearched] = useState(false);
  const [searching, setSearching] = useState(false);

  const mutation = useMutation({
    mutationFn: (accountId: number) => projectMemberAdd(projectId, accountId),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: ["project-members", projectId],
      });
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      setResults([]);
      setKeyword("");
      setSearched(false);
      onOpenChange(false);
    },
  });

  const search = async () => {
    const value = keyword.trim();
    if (value === "") return;
    setSearching(true);
    try {
      const response = await projectMemberSearch(projectId, value);
      setResults(response.data.items);
      setSearched(true);
    } finally {
      setSearching(false);
    }
  };

  const close = (next: boolean) => {
    if (!next) {
      setResults([]);
      setKeyword("");
      setSearched(false);
      mutation.reset();
    }
    onOpenChange(next);
  };

  return (
    <EditorDialog
      open={open}
      onOpenChange={close}
      title={t("members.add")}
      description={t("dialog.memberHint")}
    >
      <div className="space-y-3">
        <div className="flex gap-2">
          <Input
            aria-label={t("members.keyword")}
            placeholder={t("members.keyword")}
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                void search();
              }
            }}
          />
          <Button
            variant="outline"
            disabled={searching || keyword.trim() === ""}
            onClick={() => void search()}
          >
            {t("members.search")}
          </Button>
        </div>
        {results.map((account) => (
          <div
            key={account.accountId}
            className="flex items-center justify-between gap-2 rounded-md border p-2"
          >
            <span>
              {account.name}
              {account.email ? ` · ${account.email}` : ""}
            </span>
            <Button
              disabled={mutation.isPending}
              onClick={() => mutation.mutate(account.accountId)}
            >
              {t("members.add")}
            </Button>
          </div>
        ))}
        {searched && results.length === 0 && (
          <p className="text-sm text-muted-foreground">
            {t("members.noResult")}
          </p>
        )}
        {mutation.isError && <p role="alert">{t("members.alreadyMember")}</p>}
        <div className="flex justify-end">
          <Button type="button" variant="outline" onClick={() => close(false)}>
            {t("close")}
          </Button>
        </div>
      </div>
    </EditorDialog>
  );
}
