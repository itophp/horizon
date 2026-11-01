import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { settingsEffective, settingsPreferences } from "@/lib/capabilities";
import type { ConfigEntry } from "@/api/generated/models";
import { useAuth } from "@/features/administration/auth";
import { changeLanguage } from "@/i18n";
import { ErrorBanner } from "@/features/administration/shared";
import { useTheme } from "@/app/providers/theme";
export const governanceKey = "governance";
const RuntimeContext = createContext<ConfigEntry[]>([]);
export function useConfiguration() {
  const entries = useContext(RuntimeContext);
  return {
    entries,
    value: (key: string, fallback: string | number | boolean) => {
      const value = entries.find((entry) => entry.key === key)?.value;
      if (value === undefined) return fallback;
      try {
        const parsed: unknown = JSON.parse(value);
        return typeof parsed === typeof fallback
          ? (parsed as typeof fallback)
          : fallback;
      } catch {
        return fallback;
      }
    },
  };
}
export function RuntimeProvider({ children }: PropsWithChildren) {
  const { profile } = useAuth();
  const [preferenceError, setPreferenceError] = useState<unknown>(null);
  const legacy = useRef({
    language: localStorage.getItem("baseline.language"),
    theme: localStorage.getItem("baseline.theme"),
  });
  const migrating = useRef(false);
  const client = useQueryClient();
  const { setTheme } = useTheme();
  const query = useQuery({
    queryKey: [
      governanceKey,
      "runtime",
      profile?.tenantId,
      profile?.account.id,
      profile?.activeDepartmentId,
    ],
    queryFn: () => settingsEffective(),
    enabled: !!profile,
    staleTime: 30000,
    refetchInterval: 30000,
    retry: false,
  });
  const entries = query.data?.status === 200 ? query.data.data.entries : [];
  useEffect(() => {
    if (query.data?.status !== 200) return;
    const values = query.data.data.entries;
    if (profile && !migrating.current) {
      const marker = `baseline.preferences.synced.${profile.tenantId}.${profile.account.id}`;
      if (
        localStorage.getItem(marker) !== "1" &&
        localStorage.getItem("baseline.preferences.migrated") !== "1"
      ) {
        const changes = (
          [
            ["display.language", legacy.current.language],
            ["display.theme", legacy.current.theme],
          ] as const
        )
          .filter(
            ([key, value]) =>
              value !== null && !values.find((e) => e.key === key)?.overridden,
          )
          .map(([key, value]) => ({
            key,
            operation: "set" as const,
            value: JSON.stringify(value),
          }));
        if (changes.length) {
          migrating.current = true;
          void settingsPreferences({ changes })
            .then(() => {
              localStorage.setItem(marker, "1");
              localStorage.setItem("baseline.preferences.migrated", "1");
              void client.invalidateQueries({ queryKey: [governanceKey] });
            })
            .catch((error) => {
              setPreferenceError(error);
            });
          return;
        }
        localStorage.setItem(marker, "1");
        localStorage.setItem("baseline.preferences.migrated", "1");
      }
    }

    const apply = (key: string, action: (value: string) => void) => {
      const entry = values.find((v) => v.key === key);
      if (!entry) return;
      const parsed: unknown = JSON.parse(entry.value);
      if (typeof parsed === "string") action(parsed);
    };
    apply("display.theme", (v) => {
      if (v === "light" || v === "dark") setTheme(v);
    });
    apply("display.language", (v) => {
      if (v === "en" || v === "zh-CN") void changeLanguage(v, false);
    });
  }, [query.data, setTheme, profile, client]);
  useEffect(() => {
    if (!profile) return;
    const persist = (event: Event) => {
      if (!(event instanceof CustomEvent)) return;
      const data: unknown = event.detail;
      if (
        typeof data !== "object" ||
        data === null ||
        !("key" in data) ||
        !("value" in data) ||
        typeof data.key !== "string" ||
        typeof data.value !== "string"
      )
        return;
      void settingsPreferences({
        changes: [
          {
            key: data.key,
            operation: "set",
            value: JSON.stringify(data.value),
          },
        ],
      })
        .then(() => client.invalidateQueries({ queryKey: [governanceKey] }))
        .catch((error: unknown) => {
          setPreferenceError(error);
          void client.invalidateQueries({ queryKey: [governanceKey] });
        });
    };
    window.addEventListener("baseline:preference", persist);
    return () => window.removeEventListener("baseline:preference", persist);
  }, [profile, client]);
  return (
    <RuntimeContext.Provider value={entries}>
      {!!preferenceError && <ErrorBanner error={preferenceError} />}
      {children}
    </RuntimeContext.Provider>
  );
}
