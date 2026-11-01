import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { tableFeatures, useTable, type ColumnDef } from "@tanstack/react-table";
import { useSession } from "@/features/administration/auth";
import { PageHeading } from "@/features/administration/shared";
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
  articleGet,
  articlesList,
  articleSave,
  articleSubmit,
  articleAssess,
  articleDecide,
  type Article,
} from "@/lib/capabilities";
const features = tableFeatures({});
const schema = z.object({
  title: z.string().trim().min(1).max(200),
  body: z.string().trim().min(1).max(20000),
});
export function ArticlesPage() {
  const { t } = useTranslation("articles");
  const session = useSession();
  const client = useQueryClient();
  const canRead = session.permissions.includes("articles:read"),
    canWrite = session.permissions.includes("articles:write"),
    canReview = session.permissions.includes("articles:review");
  const [selected, setSelected] = useState<Article | null>(null);
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { title: "", body: "" },
  });
  const queryKey = ["articles", session.account?.id];
  const list = useQuery({
    queryKey,
    queryFn: articlesList,
    enabled: canRead,
    retry: false,
    refetchInterval: 5000,
  });
  const mutation = useMutation({
    mutationFn: async (action: () => Promise<unknown>) => action(),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey });
    },
  });
  const columns = useMemo<ColumnDef<typeof features, Article>[]>(
    () => [
      { accessorKey: "title", header: t("title") },
      {
        accessorKey: "status",
        header: t("status"),
        cell: ({ row }) => t(`states.${row.original.status}`),
      },
      {
        accessorKey: "reviewCode",
        header: t("reason"),
        cell: ({ row }) =>
          row.original.reviewCode ? t(`codes.${row.original.reviewCode}`) : "—",
      },
      {
        id: "actions",
        header: t("actions"),
        cell: ({ row }) => (
          <Button
            variant="outline"
            disabled={mutation.isPending}
            onClick={() =>
              mutation.mutate(async () => {
                const result = await articleGet(row.original.id);
                setSelected(result.data);
                form.reset({
                  title: result.data.title,
                  body: result.data.body ?? "",
                });
              })
            }
          >
            {t("open")}
          </Button>
        ),
      },
    ],
    [t, mutation, form],
  );
  const table = useTable({
    features,
    data: list.data?.data.items ?? [],
    columns,
  });
  const editable =
    canWrite &&
    (!selected ||
      selected.status === "draft" ||
      selected.status === "rejected");
  if (!canRead) return <p role="alert">{t("denied")}</p>;
  return (
    <section className="space-y-6">
      <PageHeading title={t("heading")} subtitle={t("description")} />
      <p>{t("policy")}</p>
      {(list.isError || mutation.isError) && <p role="alert">{t("error")}</p>}
      {list.isPending ? (
        <p>{t("loading")}</p>
      ) : (
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((g) => (
              <TableRow key={g.id}>
                {g.headers.map((h) => (
                  <TableHead key={h.id}>
                    <table.FlexRender header={h} />
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.map((row) => (
              <TableRow key={row.id}>
                {row.getAllCells().map((cell) => (
                  <TableCell key={cell.id}>
                    <table.FlexRender cell={cell} />
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      {canWrite && (
        <Button
          variant="outline"
          onClick={() => {
            setSelected(null);
            form.reset({ title: "", body: "" });
          }}
        >
          {t("new")}
        </Button>
      )}
      <form
        className="space-y-4"
        onSubmit={form.handleSubmit((values) =>
          mutation.mutate(async () => {
            await articleSave({
              ...values,
              id: selected?.id,
              revision: selected?.revision,
            });
            setSelected(null);
            form.reset();
          }),
        )}
      >
        <label htmlFor="article-title">{t("title")}</label>
        <Input
          id="article-title"
          disabled={!editable}
          {...form.register("title")}
        />
        <label htmlFor="article-body">{t("body")}</label>
        <textarea
          id="article-body"
          className="w-full min-h-64 rounded-md border p-3"
          disabled={!editable}
          {...form.register("body")}
        />
        {Object.keys(form.formState.errors).length > 0 && (
          <p role="alert">{t("invalid")}</p>
        )}
        {editable && (
          <Button type="submit" disabled={mutation.isPending}>
            {t("save")}
          </Button>
        )}
      </form>
      {selected && (
        <div className="flex gap-3">
          {canWrite && selected.status === "draft" && (
            <Button
              disabled={mutation.isPending || form.formState.isDirty}
              onClick={() =>
                mutation.mutate(async () => {
                  await articleSubmit(selected.id, selected.revision);
                  setSelected(null);
                  form.reset();
                })
              }
            >
              {t("submit")}
            </Button>
          )}
          {canWrite && selected.status === "pending_ai" && (
            <Button
              disabled={mutation.isPending}
              onClick={() =>
                mutation.mutate(async () => {
                  await articleAssess(selected.id);
                  setSelected(null);
                })
              }
            >
              {t("assess")}
            </Button>
          )}
          {canReview && selected.status === "pending_review" && (
            <>
              <Button
                disabled={mutation.isPending}
                onClick={() =>
                  mutation.mutate(async () => {
                    await articleDecide(selected.id, selected.revision, true);
                    setSelected(null);
                  })
                }
              >
                {t("approve")}
              </Button>
              <Button
                variant="outline"
                disabled={mutation.isPending}
                onClick={() =>
                  mutation.mutate(async () => {
                    await articleDecide(selected.id, selected.revision, false);
                    setSelected(null);
                  })
                }
              >
                {t("reject")}
              </Button>
            </>
          )}
        </div>
      )}
    </section>
  );
}
