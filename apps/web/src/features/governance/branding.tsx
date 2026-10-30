import { useEffect } from "react";
import { changeLanguage } from "@/i18n";
import { useTheme } from "@/app/providers/theme";
import { useQuery } from "@tanstack/react-query";
import {
  brandingGet,
  brandingLogo,
  settingsLogo,
} from "@/api/generated/baseline";
import { useAuth } from "@/features/administration/auth";
import { useConfiguration } from "./runtime";
export function useBranding() {
  const auth = useAuth();
  const runtime = useConfiguration();
  const { setTheme } = useTheme();
  const q = useQuery({
    queryKey: ["public-branding"],
    queryFn: () => brandingGet(),
    staleTime: 30000,
    retry: false,
    enabled: !auth.profile,
  });
  useEffect(() => {
    if (auth.profile || q.data?.status !== 200) return;
    for (const entry of q.data.data.entries) {
      if (entry.key !== "display.language" && entry.key !== "display.theme")
        continue;
      const value: unknown = JSON.parse(entry.value);
      if (
        entry.key === "display.language" &&
        localStorage.getItem("baseline.language") === null &&
        (value === "en" || value === "zh-CN")
      )
        void changeLanguage(value, false);
      if (
        entry.key === "display.theme" &&
        localStorage.getItem("baseline.theme") === null &&
        (value === "dark" || value === "light")
      )
        setTheme(value);
    }
  }, [q.data, auth.profile, setTheme]);
  const entries = auth.profile
    ? runtime.entries
    : q.data?.status === 200
      ? q.data.data.entries
      : [];
  return {
    text: (key: string, fallback: string) => {
      const entry = entries.find((e) => e.key === key);
      if (
        !entry ||
        entry.source === "default" ||
        (entry.source === "public" && !entry.configured)
      )
        return fallback;
      const v: unknown = JSON.parse(entry.value);
      return typeof v === "string" ? v : fallback;
    },
    logo: entries.find((e) => e.key === "branding.logo")?.value ?? "0",
  };
}
export function PublishedLogo() {
  const brand = useBranding();
  const { profile } = useAuth();
  const q = useQuery({
    queryKey: [
      "published-branding-logo",
      profile?.tenantId,
      profile?.account.id,
      profile?.activeDepartmentId,
      brand.logo,
    ],
    queryFn: async () => {
      if (profile) {
        const r = await settingsLogo();
        if (r.status === 200) return r.data;
        return null;
      }
      const r = await brandingLogo();
      return r.status === 200 ? r.data : null;
    },
    enabled: Number(brand.logo) > 0,
    retry: false,
    staleTime: 60000,
  });
  return q.data ? (
    <img
      className="gov-brand-logo"
      alt=""
      src={`data:${q.data.mime};base64,${q.data.content}`}
    />
  ) : null;
}
