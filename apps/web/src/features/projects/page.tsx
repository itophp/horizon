import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
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
  projectCreate,
  projectGet,
  projectMembers,
  projectPlans,
  projectStatusChange,
  projectUpdate,
  projectsList,
} from "@/lib/capabilities";
import { EnumSelect, targets } from "./fields";
import { MembersPanel } from "./members";
import { PlansPanel } from "./plans";
import { TasksPanel } from "./tasks";

const priorityValues = ["low", "normal", "high", "urgent"] as const;
const projectStatusValues = [
  "active",
  "paused",
  "completed",
  "archived",
] as const;

const dateRule = z
  .string()
  .refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value));

const createSchema = z.object({
  code: z.string().trim().min(1).max(32),
  name: z.string().trim().min(1).max(120),
  priority: z.enum(priorityValues),
  dueDate: dateRule,
});
type CreateValues = z.infer<typeof createSchema>;

const editSchema = createSchema.omit({ code: true });
type EditValues = z.infer<typeof editSchema>;

// projectTransitions mirrors the declared state machine; archiving is offered
// only to the owner and the server enforces that again.
const projectTransitions: Record<string, string[]> = {
  active: ["paused", "completed", "archived"],
  paused: ["active", "completed", "archived"],
  completed: ["archived"],
  archived: [],
};

export function ProjectsPage() {
  const { t } = useTranslation("projects");
  const session = useSession();
  const client = useQueryClient();
  const canRead = session.permissions.includes("projects:read");
  const canWrite = session.permissions.includes("projects:write");
  const canCreate = session.permissions.includes("projects:admin");

  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [keyword, setKeyword] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [creating, setCreating] = useState(false);

  const createForm = useForm<CreateValues>({
    resolver: zodResolver(createSchema),
    defaultValues: { code: "", name: "", priority: "normal", dueDate: "" },
  });
  const editForm = useForm<EditValues>({
    resolver: zodResolver(editSchema),
    defaultValues: { name: "", priority: "normal", dueDate: "" },
  });

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

  const detail = useQuery({
    queryKey: ["project", selectedId],
    queryFn: () => projectGet(selectedId as number),
    enabled: canRead && selectedId !== null,
    retry: false,
  });

  const members = useQuery({
    queryKey: ["project-members", selectedId],
    queryFn: () => projectMembers(selectedId as number),
    enabled: selectedId !== null,
    retry: false,
  });

  const plans = useQuery({
    queryKey: ["project-plans", selectedId],
    queryFn: () => projectPlans(selectedId as number),
    enabled: selectedId !== null,
    retry: false,
  });

  const refresh = async (id: number) => {
    await client.invalidateQueries({ queryKey: ["projects"] });
    await client.invalidateQueries({ queryKey: ["project", id] });
    await client.invalidateQueries({ queryKey: ["project-members", id] });
    await client.invalidateQueries({ queryKey: ["project-plans", id] });
    await client.invalidateQueries({ queryKey: ["project-tasks", id] });
  };

  const mutation = useMutation({
    mutationFn: async (action: () => Promise<unknown>) => action(),
    onSuccess: async () => {
      if (selectedId !== null) await refresh(selectedId);
      else await client.invalidateQueries({ queryKey: ["projects"] });
    },
  });

  if (!canRead) return <p role="alert">{t("denied")}</p>;

  const project = detail.data?.data;
  const memberItems = members.data?.data.items ?? [];
  const planItems = plans.data?.data.items ?? [];
  const archived = project?.status === "archived";
  const isOwner = project?.ownerId === session.account?.id;

  return (
    <section className="space-y-6">
      <PageHeading title={t("heading")} subtitle={t("description")}>
        {canCreate && (
          <Button
            onClick={() => {
              setCreating((value) => !value);
              createForm.reset();
            }}
          >
            {t("create")}
          </Button>
        )}
      </PageHeading>
      <p>{t("policy")}</p>
      {(projects.isError || mutation.isError) && (
        <p role="alert">{t("error")}</p>
      )}

      <div className="flex flex-wrap items-end gap-2">
        <div className="flex-1">
          <label htmlFor="project-search">{t("name")}</label>
          <Input
            id="project-search"
            placeholder={`${t("code")} / ${t("name")}`}
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
              ...projectStatusValues.map((value) => ({
                value,
                label: t(`states.${value}`),
              })),
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
                    disabled={mutation.isPending}
                    onClick={() => {
                      setSelectedId(row.id);
                      setCreating(false);
                      editForm.reset({
                        name: row.name,
                        priority: row.priority as EditValues["priority"],
                        dueDate: row.dueDate,
                      });
                    }}
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

      {canCreate && creating && (
        <form
          className="space-y-3 rounded-md border p-4"
          onSubmit={createForm.handleSubmit((values) =>
            mutation.mutate(async () => {
              const response = await projectCreate(values);
              const id = response.data.id;
              setCreating(false);
              createForm.reset();
              setSelectedId(id);
              await client.invalidateQueries({ queryKey: ["projects"] });
            }),
          )}
        >
          <p className="font-medium">{t("create")}</p>
          <label htmlFor="project-code">{t("code")}</label>
          <Input id="project-code" {...createForm.register("code")} />

          <label htmlFor="project-name">{t("name")}</label>
          <Input id="project-name" {...createForm.register("name")} />

          <label htmlFor="project-priority">{t("priority")}</label>
          <EnumSelect
            id="project-priority"
            value={createForm.watch("priority")}
            onChange={(value) =>
              createForm.setValue("priority", value as CreateValues["priority"])
            }
            options={priorityValues.map((value) => ({
              value,
              label: t(`priorities.${value}`),
            }))}
          />

          <label htmlFor="project-due">{t("dueDate")}</label>
          <Input
            id="project-due"
            type="date"
            {...createForm.register("dueDate")}
          />

          {Object.keys(createForm.formState.errors).length > 0 && (
            <p role="alert">{t("invalid")}</p>
          )}
          <Button type="submit" disabled={mutation.isPending}>
            {t("save")}
          </Button>
        </form>
      )}

      {project && selectedId !== null && (
        <div className="space-y-6 rounded-md border p-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="text-lg font-semibold">
                {t("title", { code: project.code, name: project.name })}
              </h2>
              <p className="text-sm text-muted-foreground">
                {t("summary", {
                  owner: project.ownerName,
                  status: t(`states.${project.status}`),
                  progress: `${project.taskDoneCount}/${project.taskCount}`,
                  overdue: project.taskOverdueCount,
                })}
              </p>
            </div>
            <Button variant="outline" onClick={() => setSelectedId(null)}>
              {t("close")}
            </Button>
          </div>

          {canWrite && !archived && (
            <div className="flex flex-wrap gap-2">
              {targets(projectTransitions, project.status).map((next) => (
                <Button
                  key={next}
                  variant="outline"
                  disabled={
                    mutation.isPending || (next === "archived" && !isOwner)
                  }
                  onClick={() =>
                    mutation.mutate(async () => {
                      await projectStatusChange(
                        project.id,
                        project.status,
                        next,
                      );
                    })
                  }
                >
                  {t(`statusActions.${next}`)}
                </Button>
              ))}
            </div>
          )}

          {canWrite && !archived && (
            <form
              className="space-y-3"
              onSubmit={editForm.handleSubmit((values) =>
                mutation.mutate(async () => {
                  await projectUpdate({ id: project.id, ...values });
                }),
              )}
            >
              <p className="font-medium">{t("edit")}</p>
              <label htmlFor="project-edit-name">{t("name")}</label>
              <Input id="project-edit-name" {...editForm.register("name")} />

              <label htmlFor="project-edit-priority">{t("priority")}</label>
              <EnumSelect
                id="project-edit-priority"
                value={editForm.watch("priority")}
                onChange={(value) =>
                  editForm.setValue("priority", value as EditValues["priority"])
                }
                options={priorityValues.map((value) => ({
                  value,
                  label: t(`priorities.${value}`),
                }))}
              />

              <label htmlFor="project-edit-due">{t("dueDate")}</label>
              <Input
                id="project-edit-due"
                type="date"
                {...editForm.register("dueDate")}
              />

              {Object.keys(editForm.formState.errors).length > 0 && (
                <p role="alert">{t("invalid")}</p>
              )}
              <Button type="submit" disabled={mutation.isPending}>
                {t("save")}
              </Button>
            </form>
          )}

          <MembersPanel
            projectId={project.id}
            isOwner={isOwner}
            currentAccountId={session.account?.id ?? 0}
            archived={archived}
          />

          <PlansPanel
            projectId={project.id}
            members={memberItems}
            canWrite={canWrite}
            archived={archived}
          />

          <TasksPanel
            projectId={project.id}
            members={memberItems}
            plans={planItems}
            canWrite={canWrite}
            archived={archived}
          />
        </div>
      )}
    </section>
  );
}
