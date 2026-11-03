import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
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
 * MembersPanel is the only place where the visibility boundary is edited. The
 * server decides whether the caller may write; `isOwner` only avoids showing
 * buttons that would be refused.
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
  const [keyword, setKeyword] = useState("");
  const [results, setResults] = useState<ProjectAccount[]>([]);
  const [searching, setSearching] = useState(false);

  const members = useQuery({
    queryKey: ["project-members", projectId],
    queryFn: () => projectMembers(projectId),
    retry: false,
  });

  const refresh = async () => {
    await client.invalidateQueries({
      queryKey: ["project-members", projectId],
    });
    await client.invalidateQueries({ queryKey: ["project", projectId] });
    await client.invalidateQueries({ queryKey: ["projects"] });
  };

  const mutation = useMutation({
    mutationFn: async (action: () => Promise<unknown>) => action(),
    onSuccess: async () => {
      setResults([]);
      setKeyword("");
      await refresh();
    },
  });

  const search = async () => {
    const value = keyword.trim();
    if (value === "") return;
    setSearching(true);
    try {
      const response = await projectMemberSearch(projectId, value);
      setResults(response.data.items);
    } finally {
      setSearching(false);
    }
  };

  const editable = isOwner && !archived;
  const rows = members.data?.data.items ?? [];

  return (
    <section className="space-y-3">
      <h2 className="text-lg font-semibold">{t("members.heading")}</h2>
      <p className="text-sm text-muted-foreground">
        {t("members.description")}
      </p>
      {(members.isError || mutation.isError) && (
        <p role="alert">{t("error")}</p>
      )}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("name")}</TableHead>
            <TableHead>{t("members.count")}</TableHead>
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
              <TableCell>
                {row.isOwner ? t("members.ownerBadge") : row.email || "—"}
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
              <TableCell colSpan={3}>{t("empty")}</TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      {editable && (
        <div className="space-y-2">
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
          {results.map((account) => {
            const already = rows.some(
              (row) => row.accountId === account.accountId,
            );
            return (
              <div key={account.accountId} className="flex items-center gap-2">
                <span className="flex-1">
                  {account.name}
                  {account.email ? ` · ${account.email}` : ""}
                </span>
                <Button
                  variant="outline"
                  disabled={already || mutation.isPending}
                  onClick={() =>
                    mutation.mutate(async () => {
                      await projectMemberAdd(projectId, account.accountId);
                    })
                  }
                >
                  {t("members.add")}
                </Button>
              </div>
            );
          })}
          {results.length === 0 && keyword.trim() !== "" && !searching && (
            <p className="text-sm text-muted-foreground">
              {t("members.noResult")}
            </p>
          )}
        </div>
      )}
    </section>
  );
}
