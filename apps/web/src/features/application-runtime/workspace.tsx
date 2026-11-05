import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  UNSAFE_LocationContext,
  useLocation,
  useNavigate,
  useOutlet,
  NavigationType,
  type Location,
} from "react-router";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
export type WorkspaceInstance = {
  id: string;
  active: boolean;
  setGuard: (guard: () => boolean) => () => void;
};
const WorkspaceContext = createContext<WorkspaceInstance | null>(null);
export function useWorkspaceInstance() {
  return useContext(WorkspaceContext);
}
type Tab = { id: string; location: Location; outlet: ReactNode };
const safePath = (path: string) => /^\/[a-zA-Z0-9_/-]*$/.test(path);
export function WorkspaceHost() {
  const { t } = useTranslation("runtime");
  const location = useLocation();
  const outlet = useOutlet();
  const navigate = useNavigate();
  const [state, setState] = useState<{ locationKey: string; tabs: Tab[] }>(
    () => ({
      locationKey: location.key,
      tabs: [{ id: crypto.randomUUID(), location, outlet }],
    }),
  );
  const [limit, setLimit] = useState(false);
  const guards = useMemo(() => new Map<string, () => boolean>(), []);
  if (state.locationKey !== location.key) {
    const index = state.tabs.findIndex(
      (tab) => tab.location.pathname === location.pathname,
    );
    const next =
      index >= 0
        ? state.tabs.map((tab, at) =>
            at === index ? { ...tab, location, outlet } : tab,
          )
        : state.tabs.length >= 10
          ? state.tabs
          : [...state.tabs, { id: crypto.randomUUID(), location, outlet }];
    setState({ locationKey: location.key, tabs: next });
  }
  const tabs = state.tabs;
  const setTabs = (update: (previous: Tab[]) => Tab[]) =>
    setState((previous) => ({ ...previous, tabs: update(previous.tabs) }));
  const active = tabs.find(
    (tab) => tab.location.pathname === location.pathname,
  );
  // Only route identities persist, never query values, records, drafts or credentials.
  useEffect(() => {
    const routes = tabs.map((tab) => tab.location.pathname).filter(safePath);
    sessionStorage.setItem("runtime-workspace-routes", JSON.stringify(routes));
  }, [tabs]);
  const [restored] = useState<string[]>(() => {
    try {
      const raw: unknown = JSON.parse(
        sessionStorage.getItem("runtime-workspace-routes") ?? "[]",
      );
      return Array.isArray(raw)
        ? raw
            .filter(
              (value): value is string =>
                typeof value === "string" && safePath(value),
            )
            .slice(0, 10)
        : [];
    } catch {
      return [];
    }
  });
  return (
    <>
      <nav className="runtime-workspace-tabs" aria-label={t("workspace")}>
        {tabs.map((tab, index) => (
          <span key={tab.id}>
            <Button
              type="button"
              aria-current={tab.id === active?.id ? "page" : undefined}
              onClick={() =>
                navigate(tab.location.pathname + tab.location.search)
              }
            >
              {t("workspaceTab", { number: index + 1 })}
            </Button>
            <Button
              type="button"
              aria-label={t("closeTab", {
                name: t("workspaceTab", { number: index + 1 }),
              })}
              onClick={() => {
                if (guards.get(tab.id)?.() === false) return;
                setTabs((previous) =>
                  previous.filter((item) => item.id !== tab.id),
                );
                guards.delete(tab.id);
                setLimit(false);
                if (tab.id === active?.id) {
                  const next = tabs.find((item) => item.id !== tab.id);
                  navigate(
                    next ? next.location.pathname + next.location.search : "/",
                    { replace: true },
                  );
                }
              }}
            >
              <X size={14} />
            </Button>
          </span>
        ))}
        {restored
          .filter((path) => !tabs.some((tab) => tab.location.pathname === path))
          .map((path, index) => (
            <Button
              key={path}
              type="button"
              onClick={() => {
                if (tabs.length >= 10) {
                  setLimit(true);
                  return;
                }
                navigate(path);
              }}
            >
              {t("workspaceTab", { number: tabs.length + index + 1 })}
            </Button>
          ))}
      </nav>
      {(limit || (!active && tabs.length >= 10)) && (
        <p role="alert">{t("tabLimit")}</p>
      )}
      {tabs.map((tab) => (
        <WorkspaceTab
          key={tab.id}
          tab={tab}
          active={tab.id === active?.id}
          guards={guards}
        />
      ))}
      {tabs.length === 0 && outlet}
    </>
  );
}
function WorkspaceTab({
  tab,
  active,
  guards,
}: {
  tab: Tab;
  active: boolean;
  guards: Map<string, () => boolean>;
}) {
  const value = useMemo<WorkspaceInstance>(
    () => ({
      id: tab.id,
      active,
      setGuard: (guard) => {
        guards.set(tab.id, guard);
        return () => {
          if (guards.get(tab.id) === guard) guards.delete(tab.id);
        };
      },
    }),
    [tab.id, active, guards],
  );
  return (
    <div hidden={!active}>
      <UNSAFE_LocationContext.Provider
        value={{ location: tab.location, navigationType: NavigationType.Pop }}
      >
        <WorkspaceContext.Provider value={value}>
          {tab.outlet}
        </WorkspaceContext.Provider>
      </UNSAFE_LocationContext.Provider>
    </div>
  );
}
