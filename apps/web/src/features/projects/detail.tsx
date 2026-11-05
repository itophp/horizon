import { contributionResources } from "@/features/application-runtime/contributions";
import { useTranslation } from "react-i18next";
import {
  ApplicationPage,
  EmbeddedPage,
} from "@/features/application-runtime/page";
import {
  extensions,
  type RecordData,
} from "@/features/application-runtime/v2-registry";
import { useSession } from "@/features/administration/auth";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { PlansPanel } from "./plans";
import { MembersPanel } from "./members";
function Related({ record }: { record: RecordData }) {
  const session = useSession();
  const { t } = useTranslation("projects");
  const projectId = Number(record.id);
  const archived = record.status === "archived";
  return (
    <Tabs defaultValue="tasks">
      <TabsList>
        <TabsTrigger value="tasks">{t("tabs.tasks")}</TabsTrigger>
        <TabsTrigger value="plans">{t("tabs.plans")}</TabsTrigger>
        <TabsTrigger value="members">{t("tabs.members")}</TabsTrigger>
      </TabsList>
      <TabsContent value="tasks">
        <EmbeddedPage pageId="project.tasks" readOnly={archived} />
      </TabsContent>
      <TabsContent value="plans">
        <PlansPanel
          projectId={projectId}
          archived={archived}
          canWrite={session.permissions.includes("projects:write")}
        />
      </TabsContent>
      <TabsContent value="members">
        <MembersPanel
          projectId={projectId}
          archived={archived}
          isOwner={record.ownerId === session.account.id}
          currentAccountId={session.account.id}
        />
      </TabsContent>
    </Tabs>
  );
}
contributionResources.register({
  source: "com.horizon.projects",
  entry: "project.related",
  version: "1.0.0",
  type: "component",
  activate: () =>
    extensions.attach({
      id: "project.related",
      source: "com.horizon.projects",
      version: "1.0.0",
      type: "component",
      render: Related,
    }),
});
export function ProjectDetailPage() {
  return <ApplicationPage applicationId="projects" pageId="project.detail" />;
}
