import {
  createContext,
  useEffect,
  useRef,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  UNSAFE_LocationContext,
  useLocation,
  useOutlet,
  NavigationType,
  type Location,
} from "react-router";
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
export function WorkspaceHost() {
  const location = useLocation();
  const outlet = useOutlet();
  const [state, setState] = useState<{ locationKey: string; tabs: Tab[] }>(
    () => ({
      locationKey: location.key,
      tabs: [{ id: crypto.randomUUID(), location, outlet }],
    }),
  );
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
        : [...state.tabs, { id: crypto.randomUUID(), location, outlet }];
    setState({ locationKey: location.key, tabs: next });
  }
  const tabs = state.tabs;
  const active = tabs.find(
    (tab) => tab.location.pathname === location.pathname,
  );
  return (
    <>
      {tabs.map((tab) => (
        <WorkspaceTab
          key={tab.id}
          tab={tab}
          active={tab.id === active?.id}
          guards={guards}
        />
      ))}
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
  const queryClient = useQueryClient();
  const wasActive = useRef(active);
  useEffect(() => {
    if (active && !wasActive.current) {
      void queryClient.invalidateQueries({ refetchType: "active" });
    }
    wasActive.current = active;
  }, [active, queryClient]);
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
