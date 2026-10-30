import { useBranding, PublishedLogo } from "@/features/governance/branding";
import { useState } from "react";
import { Navigate, useNavigate, useLocation } from "react-router";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import {
  ArrowRight,
  Eye,
  EyeOff,
  LockKeyhole,
  Network,
  ShieldCheck,
  Layers3,
  Moon,
  Sun,
  LoaderCircle,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTheme } from "@/app/providers/theme";
import { changeLanguage } from "@/i18n";
import { useAuth } from "./auth";
import { BrandMark, ErrorBanner, validationText } from "./shared";
const schema = z.object({
  username: z.string().min(3, { message: "required" }).max(80),
  password: z.string().min(1, { message: "required" }).max(128),
});
export function LoginPage() {
  const { t, i18n } = useTranslation();
  const brand = useBranding();
  const auth = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const state: unknown = location.state;
  const requested =
    typeof state === "object" && state !== null && "returnTo" in state
      ? state.returnTo
      : undefined;
  const destination =
    typeof requested === "string" &&
    requested.startsWith("/") &&
    !requested.startsWith("//") &&
    !requested.startsWith("/login")
      ? requested
      : "/";
  const { theme, toggle } = useTheme();
  const [visible, setVisible] = useState(false);
  const [help, setHelp] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: { username: "", password: "" },
  });
  if (auth.profile) return <Navigate to={destination} replace />;
  return (
    <main className="login-screen">
      <div className="login-container">
        <section className="login-story">
          <LinkBrand />
          <div className="login-story-content">
            <span className="login-eyebrow">
              <i />
              {t("admin:login.eyebrow")}
            </span>
            <h1>
              {brand.text("branding.loginTitle", t("admin:login.headline"))}
            </h1>
            <p>
              {brand.text(
                "branding.loginDescription",
                t("admin:login.description"),
              )}
            </p>
            {brand.text("branding.notice", "") && (
              <p>{brand.text("branding.notice", "")}</p>
            )}
            <div className="login-illustration" aria-hidden="true">
              <div className="orbit orbit-one">
                <svg
                  className="orbit-signal"
                  viewBox="0 0 100 100"
                  preserveAspectRatio="none"
                >
                  <ellipse
                    cx="50"
                    cy="50"
                    rx="49.5"
                    ry="49.5"
                    pathLength="100"
                    vectorEffect="non-scaling-stroke"
                  />
                </svg>
              </div>
              <div className="orbit orbit-two">
                <svg
                  className="orbit-signal"
                  viewBox="0 0 100 100"
                  preserveAspectRatio="none"
                >
                  <ellipse
                    cx="50"
                    cy="50"
                    rx="49.5"
                    ry="49.5"
                    pathLength="100"
                    vectorEffect="non-scaling-stroke"
                  />
                </svg>
              </div>
              <div className="orbit orbit-three">
                <svg
                  className="orbit-signal"
                  viewBox="0 0 100 100"
                  preserveAspectRatio="none"
                >
                  <ellipse
                    cx="50"
                    cy="50"
                    rx="49.5"
                    ry="49.5"
                    pathLength="100"
                    vectorEffect="non-scaling-stroke"
                  />
                </svg>
              </div>
              <div className="orbit-grid" />
              <div className="organization-core">
                <Network size={37} />
              </div>
              <span className="orbit-node node-a">
                <BuildingIcon />
              </span>
              <span className="orbit-node node-b">
                <UsersIcon />
              </span>
              <span className="orbit-node node-c">
                <ShieldCheck size={23} />
              </span>
              <span className="orbit-dot dot-a" />
              <span className="orbit-dot dot-b" />
            </div>
          </div>
          <div className="login-features">
            <span>
              <Network size={15} />
              {t("admin:login.feature1")}
            </span>
            <span>
              <ShieldCheck size={15} />
              {t("admin:login.feature2")}
            </span>
            <span>
              <Layers3 size={15} />
              {t("admin:login.feature3")}
            </span>
          </div>
        </section>
        <section className="login-form-side">
          <div className="login-controls">
            <select
              aria-label={t("common:language")}
              value={i18n.resolvedLanguage}
              onChange={(event) => {
                void changeLanguage(event.target.value);
              }}
            >
              <option value="en">{t("admin:locale.en")}</option>
              <option value="zh-CN">{t("admin:locale.zh")}</option>
            </select>
            <button
              className="icon-button"
              aria-label={t("admin:theme.toggle")}
              onClick={toggle}
            >
              {theme === "dark" ? <Sun size={17} /> : <Moon size={17} />}
            </button>
          </div>
          <div className="login-form-wrap">
            <div className="login-lock">
              <LockKeyhole size={22} />
            </div>
            <h2>{t("admin:login.welcome")}</h2>
            <p className="login-subtitle">{t("admin:login.subtitle")}</p>
            <form
              noValidate
              onSubmit={form.handleSubmit(async (values) => {
                setError(null);
                try {
                  await auth.signIn(values);
                  navigate(destination, { replace: true });
                } catch (cause) {
                  setError(cause);
                }
              })}
            >
              {!!error && <ErrorBanner error={error} />}
              <label className="login-field">
                <span id="login-account-label">
                  {t("admin:login.username")}
                </span>
                <Input
                  autoComplete="username"
                  aria-labelledby="login-account-label"
                  aria-describedby={
                    form.formState.errors.username
                      ? "login-account-error"
                      : undefined
                  }
                  placeholder={t("admin:login.usernamePlaceholder")}
                  aria-invalid={!!form.formState.errors.username}
                  {...form.register("username")}
                />
                {form.formState.errors.username && (
                  <small
                    id="login-account-error"
                    className="field-error"
                    role="alert"
                  >
                    {validationText(form.formState.errors.username.message, t)}
                  </small>
                )}
              </label>
              <div className="password-label">
                <label htmlFor="sign-in-password">
                  {t("admin:login.password")}
                </label>
                <button
                  type="button"
                  onClick={() => setHelp((value) => !value)}
                >
                  {t("admin:login.forgot")}
                </button>
              </div>
              <div className="password-input">
                <Input
                  id="sign-in-password"
                  type={visible ? "text" : "password"}
                  autoComplete="current-password"
                  placeholder={t("admin:login.passwordPlaceholder")}
                  aria-invalid={!!form.formState.errors.password}
                  {...form.register("password")}
                />
                <button
                  type="button"
                  aria-label={
                    visible
                      ? t("admin:login.hidePassword")
                      : t("admin:login.showPassword")
                  }
                  onClick={() => setVisible((value) => !value)}
                >
                  {visible ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
              {form.formState.errors.password && (
                <small className="field-error" role="alert">
                  {validationText(form.formState.errors.password.message, t)}
                </small>
              )}
              {help && (
                <p className="login-help" role="status">
                  {t("admin:login.help")}
                </p>
              )}
              <Button
                className="login-submit"
                type="submit"
                disabled={form.formState.isSubmitting}
              >
                {form.formState.isSubmitting ? (
                  <LoaderCircle size={16} className="animate-spin" />
                ) : (
                  t("admin:login.signIn")
                )}
                {!form.formState.isSubmitting && <ArrowRight size={16} />}
              </Button>
            </form>
            <div className="login-secure">
              <LockKeyhole size={12} />
              {t("admin:login.secure")}
            </div>
          </div>
          <div className="login-form-footer">
            <span>
              {t("admin:login.copyright", { year: new Date().getFullYear() })}
            </span>
            <span>
              {brand.text("branding.footer", t("admin:login.footer"))}
            </span>
            {brand.text("branding.contact", "") && (
              <span>{brand.text("branding.contact", "")}</span>
            )}
          </div>
        </section>
      </div>
    </main>
  );
}
function LinkBrand() {
  const { t } = useTranslation();
  const brand = useBranding();
  return (
    <div className="console-brand">
      <PublishedLogo />
      <BrandMark />
      <div>
        <strong>{brand.text("branding.name", t("admin:brand"))}</strong>
        <span>{brand.text("branding.tagline", t("admin:workspace"))}</span>
      </div>
    </div>
  );
}
function BuildingIcon() {
  return (
    <svg
      width="23"
      height="23"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
    >
      <rect x="5" y="3" width="14" height="18" rx="2" />
      <path d="M9 7h1m4 0h1M9 11h1m4 0h1M9 15h1m4 0h1m-5 6v-3h4v3" />
    </svg>
  );
}
function UsersIcon() {
  return (
    <svg
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
    >
      <circle cx="10" cy="8" r="3" />
      <path d="M4 20v-2a6 6 0 0112 0v2m1-15a3 3 0 010 6m2 9v-2a6 6 0 00-2-4" />
    </svg>
  );
}
