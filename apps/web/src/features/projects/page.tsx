import { useState } from "react";
import { useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useSession } from "@/features/administration/auth";
import { EditorDialog, PageHeading } from "@/features/administration/shared";
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
import { projectCreate, projectsList } from "@/lib/capabilities";
import { EnumSelect } from "./fields";

const priorityValues = ["low", "normal", "high", "urgent"] as const;

const createSchema = z.object({
  code: z.string().trim().min(1).max(32),
  name: z.string().trim().min(1).max(120),
  priority: z.enum(priorityValues),
  dueDate: z
    .string()
    .refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value)),
});
type CreateValues = z.infer<typeof createSchema>;

/** The list is a real page: opening a project navigates to its own detail route. */
export function ProjectsPage() {
  const { t } = useTranslation("projects");
  const session = useSession();
  const navigate = useNavigate();
  const canRead = session.permissions.includes("projects:read");
  const canCreate = session.permissions.includes("projects:admin");

  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [creating, setCreating] = useState(false);

  const projects = useQuery({
    queryKey: ["projects", keyword, statusFilter],
    queryFn: () =>
      projectsList({
        keyword: keyword.trim() === "" ? undefined : keyword.trim(),
        status: statusFilter === "all" ? undefined : statusFilter,
        limit: 50,
      }),
    enabled: canRead,
    retry: false,
  });

  if (!canRead) return <p role="alert">{t("denied")}</p>;

  return (
    <section className="space-y-6">
      <PageHeading title={t("heading")} subtitle={t("description")}>
        {canCreate && (
          <Button onClick={() => setCreating(true)}>{t("create")}</Button>
        )}
      </PageHeading>
      <p>{t("policy")}</p>
      {projects.isError && <p role="alert">{t("error")}</p>}

      <div className="flex flex-wrap items-end gap-2">
        <div className="flex-1">
          <label htmlFor="project-search">{t("name")}</label>
          <Input
            id="project-search"
            placeholder={t("searchHint")}
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
          />
        </div>
        <div>
          <label htmlFor="project-status-filter">{t("status")}</label>
          <EnumSelect
            id="project-status-filter"
            value={statusFilter}
            onChange={setStatusFilter}
            options={[
              { value: "all", label: t("all") },
              ...(["active", "paused", "completed", "archived"] as const).map(
                (value) => ({ value, label: t(`states.${value}`) }),
              ),
            ]}
          />
        </div>
      </div>

      {projects.isPending ? (
        <p>{t("loading")}</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("code")}</TableHead>
              <TableHead>{t("name")}</TableHead>
              <TableHead>{t("owner")}</TableHead>
              <TableHead>{t("priority")}</TableHead>
              <TableHead>{t("dueDate")}</TableHead>
              <TableHead>{t("status")}</TableHead>
              <TableHead>{t("progress")}</TableHead>
              <TableHead>{t("overdue")}</TableHead>
              <TableHead>{t("actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {(projects.data?.data.items ?? []).map((row) => (
              <TableRow key={row.id}>
                <TableCell>{row.code}</TableCell>
                <TableCell>{row.name}</TableCell>
                <TableCell>{row.ownerName}</TableCell>
                <TableCell>{t(`priorities.${row.priority}`)}</TableCell>
                <TableCell>{row.dueDate || t("dash")}</TableCell>
                <TableCell>{t(`states.${row.status}`)}</TableCell>
                <TableCell>
                  {t("ratio", {
                    done: row.taskDoneCount,
                    total: row.taskCount,
                  })}
                </TableCell>
                <TableCell>
                  {row.taskOverdueCount > 0 ? row.taskOverdueCount : t("dash")}
                </TableCell>
                <TableCell>
                  <Button
                    variant="outline"
                    onClick={() => navigate(`/projects/${row.id}`)}
                  >
                    {t("open")}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {(projects.data?.data.items ?? []).length === 0 && (
              <TableRow>
                <TableCell colSpan={9}>{t("empty")}</TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      )}

      <CreateProjectDialog
        open={creating}
        onOpenChange={setCreating}
        onCreated={(id) => {
          setCreating(false);
          navigate(`/projects/${id}`);
        }}
      />
    </section>
  );
}

function CreateProjectDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (id: number) => void;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const form = useForm<CreateValues>({
    resolver: zodResolver(createSchema),
    defaultValues: { code: "", name: "", priority: "normal", dueDate: "" },
  });
  const mutation = useMutation({
    mutationFn: (values: CreateValues) => projectCreate(values),
    onSuccess: async (response) => {
      await client.invalidateQueries({ queryKey: ["projects"] });
      form.reset();
      onCreated(response.data.id);
    },
  });

  return (
    <EditorDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("create")}
      description={t("dialog.projectHint")}
    >
      <form
        className="space-y-3"
        onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
      >
        <label htmlFor="new-project-code">{t("code")}</label>
        <Input id="new-project-code" {...form.register("code")} />

        <label htmlFor="new-project-name">{t("name")}</label>
        <Input id="new-project-name" {...form.register("name")} />

        <label htmlFor="new-project-priority">{t("priority")}</label>
        <EnumSelect
          id="new-project-priority"
          value={form.watch("priority")}
          onChange={(value) =>
            form.setValue("priority", value as CreateValues["priority"])
          }
          options={priorityValues.map((value) => ({
            value,
            label: t(`priorities.${value}`),
          }))}
        />

        <label htmlFor="new-project-due">{t("dueDate")}</label>
        <Input id="new-project-due" type="date" {...form.register("dueDate")} />

        {Object.keys(form.formState.errors).length > 0 && (
          <p role="alert">{t("invalid")}</p>
        )}
        {mutation.isError && <p role="alert">{t("error")}</p>}
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
          >
            {t("cancel")}
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            {t("save")}
          </Button>
        </div>
      </form>
    </EditorDialog>
  );
}
