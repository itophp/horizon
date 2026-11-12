import { ErpWorkspace } from "./erp-workspace";
import { createRoot } from "react-dom/client";
import { useState } from "react";
import { BrowserRouter, Link, useSearchParams } from "react-router";
import { I18nextProvider, useTranslation } from "react-i18next";
import {
  Layers,
  SunMoon,
  Languages,
  BookOpen,
  LayoutDashboard,
  List,
  FileText,
  Settings,
  CheckSquare,
  ChartNoAxesCombined,
  FolderOpen,
  ClipboardList,
  SquareKanban,
  MousePointerClick,
} from "lucide-react";
import { i18n, changeLanguage } from "@/i18n";
import { PageHeading } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import {
  Breadcrumb,
  BreadcrumbList,
  BreadcrumbItem,
  BreadcrumbSeparator,
  BreadcrumbPage,
} from "@/components/ui/breadcrumb";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { Choice } from "./primitives";
import { patternIds, states, projects } from "./catalog";
import { ProjectForm } from "./form";
import { Detail } from "./views";
import { ListPatterns } from "./list-patterns";
import { SettingsPattern } from "./settings-pattern";
import { ReviewPattern } from "./review-pattern";
import { ResourcePattern } from "./resource-pattern";
import { DashboardPattern } from "./dashboard-pattern";
import { WorkViews } from "./work-views";
import { InteractionLab } from "./interaction-lab";
import "@/styles/index.css";
import "@/styles/admin.css";
import "./showcase.css";
import "./page-patterns.css";
import "./erp.css";
import "./advanced-patterns.css";
const patternIcons = {
  dashboard: LayoutDashboard,
  listing: List,
  recordDetail: FileText,
  entryForm: ClipboardList,
  preferences: Settings,
  reviewDesk: CheckSquare,
  reporting: ChartNoAxesCombined,
  assets: FolderOpen,
  workViews: SquareKanban,
  interactionLab: MousePointerClick,
} as const;
const patterns = patternIds.map((id) => ({ id, icon: patternIcons[id] }));
function Showcase() {
  const { t } = useTranslation("showcase");
  const [q, setQ] = useSearchParams();
  const requested =
    q.get("pattern") ?? (q.has("record") ? "recordDetail" : "dashboard");
  const pattern = patterns.some((p) => p.id === requested)
    ? requested
    : "dashboard";
  const state = q.get("state") ?? "normal";
  const record = q.get("record") ?? "0";
  const [dark, setDark] = useState(
    document.documentElement.classList.contains("dark"),
  );
  const [guide, setGuide] = useState(false);
  const [edit, setEdit] = useState(false);
  const [notice, setNotice] = useState("");
  const [formMode, setFormMode] = useState("createForm");
  const [created, setCreated] = useState<string[]>([]);
  const [renamed, setRenamed] = useState<Record<string, string>>({});
  const title =
    renamed[record] ??
    created[Number(record)] ??
    projects[(Number(record) - created.length) % projects.length] ??
    record;
  function update(key: string, value: string) {
    const next = new URLSearchParams(q);
    next.set(key, value);
    setQ(next);
  }
  let content;
  if (["loading", "skeleton"].includes(state))
    content = (
      <div className="sc-skeleton" aria-busy="true" aria-label={t(state)}>
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    );
  else if (["empty", "error", "network", "denied"].includes(state))
    content = (
      <div className="sc-empty">
        <Layers size={36} />
        <h2>{t(state)}</h2>
        <p>{t(state === "empty" ? "emptyHint" : "errorHint")}</p>
        <Button onClick={() => update("state", "normal")}>{t("retry")}</Button>
      </div>
    );
  else if (pattern === "listing")
    content = (
      <ListPatterns
        created={created}
        renamed={renamed}
        onNew={() => setEdit(true)}
      />
    );
  else if (pattern === "recordDetail")
    content = (
      <>
        <Link className="sc-back" to="?pattern=listing">
          {t("back")}
        </Link>
        <Detail name={title} onEdit={() => setEdit(true)} />
      </>
    );
  else if (pattern === "entryForm")
    content = (
      <>
        <Tabs value={formMode} onValueChange={(v) => setFormMode(String(v))}>
          <TabsList variant="line">
            {["createForm", "editForm", "wizardForm"].map((key) => (
              <TabsTrigger value={key} key={key}>
                {t(key)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
        <ProjectForm
          key={formMode}
          variant={formMode === "wizardForm" ? "wizard" : "single"}
          edit={formMode === "editForm"}
          onSaved={setNotice}
        />
      </>
    );
  else if (pattern === "preferences") content = <SettingsPattern />;
  else if (pattern === "reviewDesk") content = <ReviewPattern />;
  else if (pattern === "assets") content = <ResourcePattern />;
  else if (pattern === "workViews") content = <WorkViews />;
  else if (pattern === "interactionLab") content = <InteractionLab />;
  else if (pattern === "dashboard") content = <ErpWorkspace />;
  else content = <DashboardPattern analytics={pattern === "reporting"} />;
  return (
    <div className="sc-app">
      <aside className="sc-sidebar">
        <Link className="sc-brand" to="?pattern=dashboard">
          <Layers size={24} />
          <span>
            {t("brand")}
            <small>{t("patternLibrary")}</small>
          </span>
        </Link>
        <p className="sc-nav-label">{t("layoutRule")}</p>
        <nav aria-label={t("patternLibrary")}>
          {patterns.map(({ id, icon: Icon }) => (
            <Link
              key={id}
              to={`?pattern=${id}`}
              aria-current={pattern === id ? "page" : undefined}
            >
              <Icon size={16} />
              <span>{t(id)}</span>
            </Link>
          ))}
        </nav>
        <div className="sc-sidebar-bottom">
          <p>{t("catalogue")}</p>
          <Button variant="ghost" onClick={() => setGuide(true)}>
            <BookOpen size={14} />
            {t("designGuide")}
          </Button>
        </div>
      </aside>
      <main className="sc-main">
        <header className="sc-topbar">
          <Breadcrumb>
            <BreadcrumbList>
              <BreadcrumbItem>{t("patternLibrary")}</BreadcrumbItem>
              <BreadcrumbSeparator />
              <BreadcrumbItem>
                <BreadcrumbPage>{t(pattern)}</BreadcrumbPage>
              </BreadcrumbItem>
            </BreadcrumbList>
          </Breadcrumb>
          <div>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("designGuide")}
              onClick={() => setGuide(true)}
            >
              <BookOpen size={16} />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("theme")}
              onClick={() => {
                setDark(!dark);
                document.documentElement.classList.toggle("dark", !dark);
              }}
            >
              <SunMoon size={16} />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("language")}
              onClick={() =>
                void changeLanguage(i18n.language === "en" ? "zh-CN" : "en")
              }
            >
              <Languages size={16} />
            </Button>
          </div>
        </header>
        <div className="sc-content">
          <PageHeading
            title={t(pattern === "dashboard" ? "erpTitle" : pattern)}
            subtitle={t("patternIntro")}
          >
            <Choice
              labelKey="state"
              value={state}
              options={states.filter(
                (value) => !["long", "large", "multilingual"].includes(value),
              )}
              onChange={(v) => update("state", v)}
            />
          </PageHeading>
          {notice && (
            <Alert className="sc-save-notice">
              <AlertTitle>{t("success")}</AlertTitle>
              <AlertDescription>{notice}</AlertDescription>
              <Button variant="ghost" onClick={() => setNotice("")}>
                {t("close")}
              </Button>
            </Alert>
          )}
          {state === "partial" && (
            <Alert>
              <AlertTitle>{t("partial")}</AlertTitle>
              <AlertDescription>{t("errorHint")}</AlertDescription>
            </Alert>
          )}
          {["readonly", "disabled"].includes(state) && (
            <p className="sc-note">{t(state)}</p>
          )}
          <fieldset
            disabled={["readonly", "disabled"].includes(state)}
            className="sc-page-body"
          >
            {content}
          </fieldset>
          <footer className="sc-footnote">
            <span>{t("mock")}</span>
            <span>{t("tokenText")}</span>
          </footer>
        </div>
      </main>
      <Dialog open={edit} onOpenChange={setEdit}>
        <DialogContent className="sc-dialog">
          <DialogHeader>
            <DialogTitle>
              {t(pattern === "listing" ? "new" : "editForm")}
            </DialogTitle>
            <DialogDescription>{t("localOnly")}</DialogDescription>
          </DialogHeader>
          <ProjectForm
            edit={pattern !== "listing"}
            onSaved={(message, name) => {
              if (name) {
                if (pattern === "listing") setCreated([name, ...created]);
                else setRenamed({ ...renamed, [record]: name });
              }
              setNotice(message);
              setEdit(false);
            }}
          />
        </DialogContent>
      </Dialog>
      <Sheet open={guide} onOpenChange={setGuide}>
        <SheetContent showCloseButton={false} className="sc-resource-sheet">
          <SheetHeader>
            <SheetTitle>{t("designGuide")}</SheetTitle>
            <SheetDescription>{t("patternIntro")}</SheetDescription>
          </SheetHeader>
          {["layout", "component", "interaction"].map((key) => (
            <section key={key}>
              <h2>{t(key + "Rule")}</h2>
              <p>{t(`guides.${pattern}.${key}`)}</p>
            </section>
          ))}
          <Button variant="outline" onClick={() => setGuide(false)}>
            {t("close")}
          </Button>
        </SheetContent>
      </Sheet>
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <I18nextProvider i18n={i18n}>
    <BrowserRouter useTransitions={false}>
      <Showcase />
    </BrowserRouter>
  </I18nextProvider>,
);
