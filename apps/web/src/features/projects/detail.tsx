import { useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useSession } from "@/features/administration/auth";
import { EditorDialog, PageHeading } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  projectGet,
  projectStatusChange,
  projectUpdate,
  type Project,
} from "@/lib/capabilities";
import { EnumSelect, targets } from "./fields";
import { MembersPanel } from "./members";
import { PlansPanel } from "./plans";
import { TasksPanel } from "./tasks";

const priorityValues = ["low", "normal", "high", "urgent"] as const;

// projectTransitions mirrors the declared state machine; archiving is offered to
// the owner only and the server enforces it again.
const projectTransitions: Record<string, string[]> = {
  active: ["paused", "completed", "archived"],
  paused: ["active", "completed", "archived"],
  completed: ["archived"],
  archived: [],
};

const editSchema = z.object({
  name: z.string().trim().min(1).max(120),
  priority: z.enum(priorityValues),
  dueDate: z
    .string()
    .refine((value) => value === "" || /^\d{4}-\d{2}-\d{2}$/.test(value)),
});
type EditValues = z.infer<typeof editSchema>;

/**
 * ProjectDetailPage owns one project. Plans, tasks and members live in their own
 * tabs whose editors are dialogs, so the page does not turn into one long form.
 */
export function ProjectDetailPage() {
  const { t } = useTranslation("projects");
  const session = useSession();
  const navigate = useNavigate();
  const client = useQueryClient();
  const params = useParams();
  const projectId = Number(params.id);
  const canWrite = session.permissions.includes("projects:write");

  const [editing, setEditing] = useState(false);

  const detail = useQuery({
    queryKey: ["project", projectId],
    queryFn: () => projectGet(projectId),
    enabled: Number.isFinite(projectId) && projectId > 0,
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: async (action: () => Promise<unknown>) => action(),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      await client.invalidateQueries({ queryKey: ["projects"] });
    },
  });

  if (!Number.isFinite(projectId) || projectId <= 0) {
    return <p role="alert">{t("notFound")}</p>;
  }
  if (detail.isPending) return <p>{t("loading")}</p>;
  if (detail.isError) return <p role="alert">{t("notFound")}</p>;

  const project = detail.data.data;
  const archived = project.status === "archived";
  const isOwner = project.ownerId === session.account?.id;

  return (
    <section className="space-y-6">
      <PageHeading
        title={t("title", { code: project.code, name: project.name })}
        subtitle={t("summary", {
          owner: project.ownerName,
          status: t(`states.${project.status}`),
          progress: `${project.taskDoneCount}/${project.taskCount}`,
          overdue: project.taskOverdueCount,
        })}
      >
        <Button variant="outline" onClick={() => navigate("/projects")}>
          {t("back")}
        </Button>
      </PageHeading>
      {mutation.isError && <p role="alert">{t("error")}</p>}

      {canWrite && !archived && (
        <div className="flex flex-wrap gap-2">
          {targets(projectTransitions, project.status).map((next) => (
            <Button
              key={next}
              variant="outline"
              disabled={mutation.isPending || (next === "archived" && !isOwner)}
              onClick={() =>
                mutation.mutate(async () => {
                  await projectStatusChange(project.id, project.status, next);
                })
              }
            >
              {t(`statusActions.${next}`)}
            </Button>
          ))}
        </div>
      )}

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">{t("tabs.overview")}</TabsTrigger>
          <TabsTrigger value="plans">{t("tabs.plans")}</TabsTrigger>
          <TabsTrigger value="tasks">{t("tabs.tasks")}</TabsTrigger>
          <TabsTrigger value="members">{t("tabs.members")}</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <Overview
            project={project}
            canEdit={canWrite && !archived}
            onEdit={() => setEditing(true)}
          />
        </TabsContent>
        <TabsContent value="plans">
          <PlansPanel
            projectId={project.id}
            canWrite={canWrite}
            archived={archived}
          />
        </TabsContent>
        <TabsContent value="tasks">
          <TasksPanel
            projectId={project.id}
            canWrite={canWrite}
            archived={archived}
          />
        </TabsContent>
        <TabsContent value="members">
          <MembersPanel
            projectId={project.id}
            isOwner={isOwner}
            currentAccountId={session.account?.id ?? 0}
            archived={archived}
          />
        </TabsContent>
      </Tabs>

      {canWrite && !archived && (
        <EditProjectDialog
          open={editing}
          onOpenChange={setEditing}
          project={project}
        />
      )}
    </section>
  );
}

function Overview({
  project,
  canEdit,
  onEdit,
}: {
  project: Project;
  canEdit: boolean;
  onEdit: () => void;
}) {
  const { t } = useTranslation("projects");
  return (
    <div className="space-y-4">
      <dl className="grid gap-3 sm:grid-cols-2">
        <div>
          <dt className="text-sm text-muted-foreground">{t("code")}</dt>
          <dd>{project.code}</dd>
        </div>
        <div>
          <dt className="text-sm text-muted-foreground">{t("name")}</dt>
          <dd>{project.name}</dd>
        </div>
        <div>
          <dt className="text-sm text-muted-foreground">{t("owner")}</dt>
          <dd>{project.ownerName}</dd>
        </div>
        <div>
          <dt className="text-sm text-muted-foreground">{t("priority")}</dt>
          <dd>{t(`priorities.${project.priority}`)}</dd>
        </div>
        <div>
          <dt className="text-sm text-muted-foreground">{t("dueDate")}</dt>
          <dd>{project.dueDate || t("dash")}</dd>
        </div>
        <div>
          <dt className="text-sm text-muted-foreground">{t("status")}</dt>
          <dd>{t(`states.${project.status}`)}</dd>
        </div>
        <div>
          <dt className="text-sm text-muted-foreground">{t("progress")}</dt>
          <dd>
            {t("ratio", {
              done: project.taskDoneCount,
              total: project.taskCount,
            })}
          </dd>
        </div>
        <div>
          <dt className="text-sm text-muted-foreground">{t("overdue")}</dt>
          <dd>{project.taskOverdueCount}</dd>
        </div>
      </dl>
      {canEdit && (
        <Button variant="outline" onClick={onEdit}>
          {t("edit")}
        </Button>
      )}
    </div>
  );
}

function EditProjectDialog({
  open,
  onOpenChange,
  project,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  project: Project;
}) {
  const { t } = useTranslation("projects");
  const client = useQueryClient();
  const form = useForm<EditValues>({
    resolver: zodResolver(editSchema),
    values: {
      name: project.name,
      priority: project.priority as EditValues["priority"],
      dueDate: project.dueDate,
    },
  });
  const mutation = useMutation({
    mutationFn: (values: EditValues) =>
      projectUpdate({ id: project.id, ...values }),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["project", project.id] });
      await client.invalidateQueries({ queryKey: ["projects"] });
      onOpenChange(false);
    },
  });

  return (
    <EditorDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("edit")}
      description={t("dialog.projectEditHint")}
    >
      <form
        className="space-y-3"
        onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
      >
        <label htmlFor="edit-project-name">{t("name")}</label>
        <Input id="edit-project-name" {...form.register("name")} />

        <label htmlFor="edit-project-priority">{t("priority")}</label>
        <EnumSelect
          id="edit-project-priority"
          value={form.watch("priority")}
          onChange={(value) =>
            form.setValue("priority", value as EditValues["priority"])
          }
          options={priorityValues.map((value) => ({
            value,
            label: t(`priorities.${value}`),
          }))}
        />

        <label htmlFor="edit-project-due">{t("dueDate")}</label>
        <Input
          id="edit-project-due"
          type="date"
          {...form.register("dueDate")}
        />

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
