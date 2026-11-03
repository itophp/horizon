import { useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useTranslation } from "react-i18next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Archive,
  ArrowLeft,
  CheckCheck,
  Pause,
  Pencil,
  Play,
} from "lucide-react";
import { useSession } from "@/features/administration/auth";
import {
  EditorDialog,
  ErrorBanner,
  Loading,
  PageHeading,
  ShortDate,
  StateBadge,
  stateTone,
  SuccessNotice,
} from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  projectGet,
  projectStatusChange,
  type Project,
} from "@/lib/capabilities";
import { targets } from "./fields";
import { ProjectForm } from "./forms";
import { MembersPanel } from "./members";
import { PlansPanel } from "./plans";
import { TasksPanel } from "./tasks";

const projectTransitions: Record<string, string[]> = {
  active: ["paused", "completed", "archived"],
  paused: ["active", "completed", "archived"],
  completed: ["archived"],
  archived: [],
};

const transitionIcons: Record<string, typeof Pause> = {
  paused: Pause,
  active: Play,
  completed: CheckCheck,
  archived: Archive,
};

/**
 * ProjectDetailPage is one project: the header holds the state commands, the
 * overview panel holds the fields, and the objects under it live in tabs.
 */
export function ProjectDetailPage() {
  const { t } = useTranslation(["projects", "admin"]);
  const session = useSession();
  const navigate = useNavigate();
  const client = useQueryClient();
  const params = useParams();
  const projectId = Number(params.id);
  const [editing, setEditing] = useState(false);
  const [notice, setNotice] = useState("");

  const can = (action: string) =>
    session.permissions.includes(`projects:${action}`);

  const query = useQuery({
    queryKey: ["project", projectId],
    queryFn: () => projectGet(projectId),
    enabled: Number.isFinite(projectId) && projectId > 0,
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: ({ project, next }: { project: Project; next: string }) =>
      projectStatusChange(project.id, project.status, next),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["project", projectId] });
      await client.invalidateQueries({ queryKey: ["projects"] });
      setNotice(t("admin:feedback.saved"));
    },
  });

  if (!Number.isFinite(projectId) || projectId <= 0) {
    return <ErrorBanner error={new Error("not_found")} />;
  }
  if (query.isPending) return <Loading />;
  if (query.isError) return <ErrorBanner error={query.error} />;

  const project = query.data.data;
  const archived = project.status === "archived";
  const isOwner = project.ownerId === session.account?.id;

  return (
    <>
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
          <ArrowLeft size={15} />
          {t("back")}
        </Button>
        {can("write") && !archived && (
          <Button onClick={() => setEditing(true)}>
            <Pencil size={15} />
            {t("editProject")}
          </Button>
        )}
      </PageHeading>
      {notice && (
        <SuccessNotice message={notice} onClose={() => setNotice("")} />
      )}

      <section className="admin-panel">
        <div className="panel-tabs">
          <button className="active">
            {t("tabs.overview")}
            <span>{t(`states.${project.status}`)}</span>
          </button>
        </div>
        <div className="detail-body">
          <dl className="detail-grid">
            <div>
              <dt>{t("code")}</dt>
              <dd>{project.code}</dd>
            </div>
            <div>
              <dt>{t("owner")}</dt>
              <dd>{project.ownerName}</dd>
            </div>
            <div>
              <dt>{t("priority")}</dt>
              <dd>{t(`priorities.${project.priority}`)}</dd>
            </div>
            <div>
              <dt>{t("dueDate")}</dt>
              <dd>
                {project.dueDate ? (
                  <ShortDate value={project.dueDate} />
                ) : (
                  t("dash")
                )}
              </dd>
            </div>
            <div>
              <dt>{t("status")}</dt>
              <dd>
                <StateBadge
                  label={t(`states.${project.status}`)}
                  tone={stateTone(project.status)}
                />
              </dd>
            </div>
            <div>
              <dt>{t("progress")}</dt>
              <dd>
                {t("ratio", {
                  done: project.taskDoneCount,
                  total: project.taskCount,
                })}
              </dd>
            </div>
            <div>
              <dt>{t("overdue")}</dt>
              <dd>{project.taskOverdueCount}</dd>
            </div>
          </dl>
          {can("write") && !archived && (
            <div className="detail-actions">
              {targets(projectTransitions, project.status).map((next) => {
                const Icon = transitionIcons[next];
                return (
                  <Button
                    key={next}
                    variant="outline"
                    disabled={
                      mutation.isPending || (next === "archived" && !isOwner)
                    }
                    onClick={() => mutation.mutate({ project, next })}
                  >
                    <Icon size={14} />
                    {t(`statusActions.${next}`)}
                  </Button>
                );
              })}
            </div>
          )}
        </div>
      </section>

      <Tabs defaultValue="plans" className="object-tabs">
        <TabsList>
          <TabsTrigger value="plans">{t("tabs.plans")}</TabsTrigger>
          <TabsTrigger value="tasks">{t("tabs.tasks")}</TabsTrigger>
          <TabsTrigger value="members">{t("tabs.members")}</TabsTrigger>
        </TabsList>
        <TabsContent value="plans">
          <PlansPanel
            projectId={project.id}
            canWrite={can("write")}
            archived={archived}
          />
        </TabsContent>
        <TabsContent value="tasks">
          <TasksPanel
            projectId={project.id}
            canWrite={can("write")}
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

      {can("write") && !archived && (
        <EditorDialog
          open={editing}
          onOpenChange={setEditing}
          title={t("editProject")}
          description={t("dialog.projectEditHint")}
          wide
        >
          <ProjectForm
            key={project.id}
            record={project}
            onCancel={() => setEditing(false)}
            onSaved={(message) => {
              setEditing(false);
              setNotice(message);
            }}
          />
        </EditorDialog>
      )}
    </>
  );
}
