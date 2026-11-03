import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { KeyRound, Trash2, UserPlus } from "lucide-react";
import {
  DataTable,
  EditorDialog,
  ErrorBanner,
  Loading,
  RowActions,
  SuccessNotice,
  type AdminColumn,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  projectMemberAdd,
  projectMemberRemove,
  projectMemberSearch,
  projectMembers,
  projectOwnerTransfer,
  type ProjectAccount,
  type ProjectMember,
} from "@/lib/capabilities";

/**
 * MembersPanel is the only place the visibility boundary is edited. Adding a
 * member is a dialog that searches the tenant directory.
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
  const { t } = useTranslation(["projects", "admin"]);
  const client = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [notice, setNotice] = useState("");

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
      setNotice(t("admin:feedback.saved"));
    },
  });

  const editable = isOwner && !archived;
  // DataTable keys rows by id; a member's identity is its account.
  const rows = (members.data?.data.items ?? []).map((member) => ({
    ...member,
    id: member.accountId,
  }));

  const columns: AdminColumn<ProjectMember & { id: number }>[] = [
    {
      id: "name",
      header: t("name"),
      cell: (member) => (
        <span>
          <strong>{member.name}</strong>
          {member.accountId === currentAccountId && (
            <small className="muted-text"> {t("selfLabel")}</small>
          )}
        </span>
      ),
    },
    {
      id: "email",
      header: t("members.email"),
      cell: (member) => member.email || t("dash"),
    },
    {
      id: "role",
      header: t("members.role"),
      cell: (member) =>
        member.isOwner ? t("members.ownerBadge") : t("members.memberBadge"),
    },
    {
      id: "actions",
      header: t("admin:fields.actions"),
      className: "actions-column",
      cell: (member) =>
        editable && !member.isOwner ? (
          <RowActions
            actions={[
              {
                label: t("members.makeOwner"),
                icon: <KeyRound size={14} />,
                onClick: () =>
                  mutation.mutate(() =>
                    projectOwnerTransfer(projectId, member.accountId),
                  ),
              },
              {
                label: t("members.remove"),
                icon: <Trash2 size={14} />,
                danger: true,
                onClick: () =>
                  mutation.mutate(() =>
                    projectMemberRemove(projectId, member.accountId),
                  ),
              },
            ]}
          />
        ) : null,
    },
  ];

  if (members.isPending) return <Loading />;

  return (
    <>
      {members.isError && (
        <ErrorBanner
          error={members.error}
          onRetry={() => {
            void members.refetch();
          }}
        />
      )}
      <section className="admin-panel">
        <div className="table-toolbar">
          <p className="muted-text">{t("members.description")}</p>
          {editable && (
            <div className="toolbar-right">
              <Button onClick={() => setAdding(true)}>
                <UserPlus size={15} />
                {t("members.add")}
              </Button>
            </div>
          )}
        </div>
        <DataTable rows={rows} columns={columns} />
      </section>

      {editable && (
        <AddMemberDialog
          open={adding}
          onClose={() => setAdding(false)}
          projectId={projectId}
          existing={rows.map((row) => row.accountId)}
          onAdded={() => {
            setAdding(false);
            setNotice(t("admin:feedback.saved"));
            void members.refetch();
          }}
        />
      )}
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}
    </>
  );
}

function AddMemberDialog({
  open,
  onClose,
  projectId,
  existing,
  onAdded,
}: {
  open: boolean;
  onClose: () => void;
  projectId: number;
  existing: number[];
  onAdded: () => void;
}) {
  const { t } = useTranslation(["projects", "admin"]);
  const [keyword, setKeyword] = useState("");
  const [results, setResults] = useState<ProjectAccount[]>([]);
  const [searched, setSearched] = useState(false);
  const [searching, setSearching] = useState(false);

  const mutation = useMutation({
    mutationFn: (accountId: number) => projectMemberAdd(projectId, accountId),
    onSuccess: () => onAdded(),
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
    onClose();
  };

  return (
    <EditorDialog
      open={open}
      onOpenChange={close}
      title={t("members.add")}
      description={t("dialog.memberHint")}
    >
      <div className="entity-form">
        {mutation.isError && <ErrorBanner error={mutation.error} />}
        <div className="editor-grid">
          <div className="editor-field field-wide">
            <label htmlFor="member-keyword">{t("members.keyword")}</label>
            <div className="member-search">
              <Input
                id="member-keyword"
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
          </div>
        </div>
        <div>
          {results.map((account) => {
            const already = existing.includes(account.accountId);
            return (
              <div key={account.accountId} className="member-result">
                <span>
                  <strong>{account.name}</strong>
                  <small>{account.email}</small>
                </span>
                <Button
                  variant="outline"
                  disabled={already || mutation.isPending}
                  onClick={() => mutation.mutate(account.accountId)}
                >
                  {already ? t("members.memberBadge") : t("members.add")}
                </Button>
              </div>
            );
          })}
          {searched && results.length === 0 && (
            <p className="muted-text">{t("members.noResult")}</p>
          )}
        </div>
        <div className="editor-footer">
          <Button variant="outline" onClick={() => close(false)}>
            {t("close")}
          </Button>
        </div>
      </div>
    </EditorDialog>
  );
}
