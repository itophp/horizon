import "./mcp.css";
import "@/features/webmcp/webmcp.css";
import {
  prepareInstall,
  installPrepared,
  diagnoseRemote,
  type BusinessClient,
} from "@/lib/mcp/connection";
import { useState } from "react";
import { Plus, ShieldAlert } from "lucide-react";
import { useController, useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  mcpManagementCreate,
  mcpManagementAdminRevoke,
  mcpManagementRotate,
  useMcpManagementCapabilities,
  useMcpManagementAudit,
  mcpManagementRevoke,
  useMcpManagementList,
  getMcpManagementListQueryKey,
} from "@/api/generated/baseline";
import { ApiError } from "@/api/client/request";
import { useAuth } from "@/features/administration/auth";
import { DataTable, EditorDialog } from "@/features/administration/shared";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const schema = z.object({
  name: z
    .string()
    .trim()
    .min(1, { message: "name" })
    .max(100, { message: "name" }),
  days: z
    .number({ message: "days" })
    .int({ message: "days" })
    .min(1, { message: "days" })
    .max(90, { message: "days" }),
  departmentId: z
    .number({ message: "department" })
    .int()
    .positive({ message: "department" }),
  tools: z.array(z.string()).min(1, { message: "tools" }),
  password: z
    .string({ message: "password" })
    .min(1, { message: "password" })
    .max(128, { message: "password" }),
});
const adminSchema = z.object({
  id: z.number({ message: "id" }).int().positive({ message: "id" }),
  password: z
    .string()
    .min(1, { message: "password" })
    .max(128, { message: "password" }),
});
// Rotating or revoking an existing token only needs the account credential.
const confirmSchema = z.object({
  password: z
    .string({ message: "password" })
    .min(1, { message: "password" })
    .max(128, { message: "password" }),
});

// Only surface reasons the API contract actually distinguishes; anything else stays generic
// instead of leaking transport details into the interface.
function failureText(error: unknown, t: TFunction) {
  const known = [
    "forbidden",
    "unauthenticated",
    "invalid",
    "unavailable",
    "rate_limited",
    "internal",
    "conflict",
  ];
  const code = error instanceof ApiError ? error.code : "";
  return t(`failure.${known.includes(code) ? code : "unknown"}`);
}

function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null;
  return (
    <small id={id} className="field-error" role="alert">
      {message}
    </small>
  );
}

export function RemoteMcpSection() {
  const { t, i18n } = useTranslation("mcp");
  const auth = useAuth();
  const query = useMcpManagementList();

  const audit = useMcpManagementAudit();
  const client = useQueryClient();
  const [secret, setSecret] = useState("");
  const [catalogBlocked, setCatalogBlocked] = useState(false);
  // The token list is the primary content of this section: issuing, rotating and
  // revoking all happen inside a dialog opened from that list instead of from
  // forms that stay on the page.
  const [creating, setCreating] = useState(false);
  const [confirming, setConfirming] = useState<{
    kind: "rotate" | "revoke";
    id: number;
    name: string;
  } | null>(null);
  const [emergencyOpen, setEmergencyOpen] = useState(false);
  const canEmergencyRevoke = auth.profile!.permissions.includes("mcp:revoke");
  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      name: "",
      password: "",
      days: 30,
      departmentId: auth.profile!.activeDepartmentId,
      tools: ["context_get", "capabilities_list"],
    },
  });
  const capabilities = useMcpManagementCapabilities({
    departmentId: useWatch({ control: form.control, name: "departmentId" }),
  });
  const availableTools =
    capabilities.data?.status === 200 ? capabilities.data.data : [];
  // Selections survive a reload of the tool catalog: a catalog that is refetching,
  // empty or unavailable must not silently drop what the operator already chose.
  const tools = useController({ control: form.control, name: "tools" });
  const refresh = () =>
    client.invalidateQueries({ queryKey: getMcpManagementListQueryKey() });
  const create = useMutation({
    mutationFn: (value: z.infer<typeof schema>) => mcpManagementCreate(value),
    onSuccess: async (result) => {
      if (result.status === 201) {
        setSecret(result.data.secret);
        setCreating(false);
      }
      form.resetField("password");
      await refresh();
    },
  });
  const confirmForm = useForm({
    resolver: zodResolver(confirmSchema),
    defaultValues: { password: "" },
  });
  const rotate = useMutation({
    mutationFn: ({ id, password }: { id: number; password: string }) =>
      mcpManagementRotate(id, { password }),
    onSuccess: async (result) => {
      if (result.status === 201) setSecret(result.data.secret);
      setConfirming(null);
      confirmForm.reset();
      await refresh();
    },
  });
  const revoke = useMutation({
    mutationFn: ({ id, password }: { id: number; password: string }) =>
      mcpManagementRevoke(id, { password }),
    onSuccess: async () => {
      setConfirming(null);
      confirmForm.reset();
      await refresh();
    },
  });
  const adminForm = useForm({
    resolver: zodResolver(adminSchema),
    defaultValues: { id: 0, password: "" },
  });
  const adminRevoke = useMutation({
    mutationFn: ({ id, password }: z.infer<typeof adminSchema>) =>
      mcpManagementAdminRevoke(id, { password }),
    onSuccess: async () => {
      setEmergencyOpen(false);
      adminForm.reset();
      await refresh();
    },
  });
  const helperForm = useForm({ defaultValues: { key: "", token: "" } });
  const [helperStatus, setHelperStatus] = useState<
    "ready" | "busy" | "installed" | "connected" | "failed"
  >("ready");
  const [prepared, setPrepared] = useState<Awaited<
    ReturnType<typeof prepareInstall>
  > | null>(null);
  async function prepare(client: BusinessClient) {
    setHelperStatus("busy");
    try {
      setPrepared(
        await prepareInstall(
          helperForm.getValues("key"),
          client,
          auth.profile!.account.id,
          form.getValues("departmentId"),
        ),
      );
      setHelperStatus("ready");
    } catch {
      setHelperStatus("failed");
    }
  }
  async function install() {
    if (!prepared) return;
    setHelperStatus("busy");
    try {
      await installPrepared(
        helperForm.getValues("key"),
        prepared.proof,
        helperForm.getValues("token"),
      );
      setHelperStatus("installed");
    } catch {
      setHelperStatus("failed");
    } finally {
      setPrepared(null);
      helperForm.reset();
    }
  }
  async function diagnose() {
    setHelperStatus("busy");
    try {
      await diagnoseRemote(helperForm.getValues("token"));
      setHelperStatus("connected");
      void audit.refetch();
    } catch {
      setHelperStatus("failed");
    } finally {
      helperForm.resetField("token");
    }
  }
  const errors = form.formState.errors;
  const tokens = query.data?.status === 200 ? query.data.data : [];
  return (
    <div className="webmcp-stack">
      <p className="muted-text">{t("description")}</p>
      <section className="admin-panel webmcp-section">
        <div className="mcp-token-head">
          <div>
            <h2>{t("tokens")}</h2>
            <p>{t("tokenLimits")}</p>
          </div>
          <div className="mcp-token-actions">
            <Button
              disabled={!!secret || create.isPending}
              onClick={() => {
                setCatalogBlocked(false);
                setCreating(true);
              }}
            >
              <Plus size={15} />
              {t("addToken")}
            </Button>
            {canEmergencyRevoke && (
              <Button
                variant="outline"
                onClick={() => {
                  adminRevoke.reset();
                  setEmergencyOpen(true);
                }}
              >
                <ShieldAlert size={15} />
                {t("emergencyRevoke")}
              </Button>
            )}
          </div>
        </div>
        <EditorDialog
          open={creating}
          onOpenChange={setCreating}
          title={t("createTitle")}
          description={t("tokenLimits")}
          wide
        >
          <form
            className="mcp-form"
            noValidate
            onSubmit={form.handleSubmit((value) => {
              if (availableTools.length === 0) {
                setCatalogBlocked(true);
                return;
              }
              setCatalogBlocked(false);
              create.mutate(value);
            })}
          >
            <div className="editor-field">
              <label htmlFor="mcp-name">{t("name")}</label>
              <Input
                id="mcp-name"
                aria-invalid={!!errors.name}
                aria-describedby={errors.name ? "mcp-name-error" : undefined}
                {...form.register("name")}
              />
              <FieldError
                id="mcp-name-error"
                message={errors.name ? t("validation.name") : undefined}
              />
            </div>
            <div className="editor-field">
              <label htmlFor="mcp-days">{t("days")}</label>
              <Input
                id="mcp-days"
                type="number"
                min={1}
                max={90}
                aria-invalid={!!errors.days}
                aria-describedby={errors.days ? "mcp-days-error" : undefined}
                {...form.register("days", { valueAsNumber: true })}
              />
              <FieldError
                id="mcp-days-error"
                message={errors.days ? t("validation.days") : undefined}
              />
            </div>
            <div className="editor-field">
              <label htmlFor="mcp-department">{t("department")}</label>
              <select
                id="mcp-department"
                aria-invalid={!!errors.departmentId}
                aria-describedby={
                  errors.departmentId ? "mcp-department-error" : undefined
                }
                {...form.register("departmentId", { valueAsNumber: true })}
              >
                {auth.profile!.account.memberships.map((m) => (
                  <option key={m.departmentId} value={m.departmentId}>
                    {m.departmentName || m.departmentId}
                  </option>
                ))}
              </select>
              <FieldError
                id="mcp-department-error"
                message={
                  errors.departmentId ? t("validation.department") : undefined
                }
              />
            </div>
            <div className="editor-field">
              <label htmlFor="mcp-password">{t("password")}</label>
              <Input
                id="mcp-password"
                type="password"
                autoComplete="current-password"
                aria-invalid={!!errors.password}
                aria-describedby={
                  errors.password ? "mcp-password-error" : undefined
                }
                {...form.register("password")}
              />
              <FieldError
                id="mcp-password-error"
                message={errors.password ? t("validation.password") : undefined}
              />
            </div>
            <fieldset
              className="mcp-tools"
              aria-invalid={!!errors.tools}
              aria-describedby={errors.tools ? "mcp-tools-error" : undefined}
            >
              <legend>{t("tools")}</legend>
              {availableTools.length === 0 ? (
                <small className="field-error" role="alert">
                  {capabilities.isPending
                    ? t("catalogLoading")
                    : t("catalogUnavailable")}
                </small>
              ) : (
                availableTools.map((tool) => (
                  <label className="mcp-tool-choice" key={tool}>
                    <input
                      type="checkbox"
                      value={tool}
                      checked={(tools.field.value ?? []).includes(tool)}
                      onChange={(event) => {
                        const selected = tools.field.value ?? [];
                        const next = event.target.checked
                          ? [...selected, tool]
                          : selected.filter((item) => item !== tool);
                        tools.field.onChange([...new Set(next)]);
                      }}
                    />
                    {t(`toolNames.${tool}`)}
                  </label>
                ))
              )}
              <FieldError
                id="mcp-tools-error"
                message={errors.tools ? t("validation.tools") : undefined}
              />
            </fieldset>
            {create.isError && (
              <p role="alert" className="form-error">
                {failureText(create.error, t)}
              </p>
            )}
            {catalogBlocked && !create.isError && (
              <p role="alert" className="form-error">
                {t("catalogBlocked")}
              </p>
            )}
            <div className="mcp-dialog-actions">
              <Button
                type="button"
                variant="outline"
                onClick={() => setCreating(false)}
              >
                {t("cancel")}
              </Button>
              <Button disabled={create.isPending} type="submit">
                {t("create")}
              </Button>
            </div>
          </form>
        </EditorDialog>
        {secret && (
          <div className="mcp-secret" role="status">
            <p>{t("secretOnce")}</p>
            <code>{secret}</code>
            <Button
              onClick={() => {
                setSecret("");
                create.reset();
                rotate.reset();
              }}
            >
              {t("dismiss")}
            </Button>
          </div>
        )}
        {query.isError && (
          <p role="alert" className="form-error">
            {failureText(query.error, t)}
          </p>
        )}
        {query.isPending && <p>{t("loading")}</p>}
        <DataTable
          rows={tokens}
          columns={[
            { id: "name", header: t("name"), cell: (token) => token.name },
            {
              id: "prefix",
              header: t("prefix"),
              cell: (token) => <code>{token.prefix}</code>,
            },
            {
              id: "department",
              header: t("department"),
              cell: (token) => token.departmentId,
            },
            {
              id: "expires",
              header: t("expires"),
              cell: (token) =>
                new Intl.DateTimeFormat(i18n.resolvedLanguage, {
                  dateStyle: "medium",
                }).format(new Date(token.expiresAt)),
            },
            {
              id: "tools",
              header: t("tools"),
              cell: (token) =>
                token.tools
                  .map((tool) => t(`toolNames.${tool}`))
                  .join(t("separator")),
            },
            {
              id: "revokedBy",
              header: t("revokedBy"),
              cell: (token) =>
                token.revokedAt
                  ? token.revokedBy || t("systemActor")
                  : t("notRevoked"),
            },
            {
              id: "actions",
              header: t("actions"),
              cell: (token) => (
                <div className="webmcp-actions">
                  <Button
                    disabled={!!secret || !!token.revokedAt || rotate.isPending}
                    onClick={() => {
                      rotate.reset();
                      confirmForm.reset();
                      setConfirming({
                        kind: "rotate",
                        id: token.id,
                        name: token.name,
                      });
                    }}
                  >
                    {t("rotate")}
                  </Button>
                  <Button
                    disabled={!!token.revokedAt || revoke.isPending}
                    onClick={() => {
                      revoke.reset();
                      confirmForm.reset();
                      setConfirming({
                        kind: "revoke",
                        id: token.id,
                        name: token.name,
                      });
                    }}
                  >
                    {token.revokedAt ? t("revoked") : t("revoke")}
                  </Button>
                </div>
              ),
            },
          ]}
        />
        <EditorDialog
          open={!!confirming}
          onOpenChange={(open) => {
            if (!open) setConfirming(null);
          }}
          title={
            confirming?.kind === "revoke"
              ? t("revokeTitle", { name: confirming.name })
              : t("rotateTitle", { name: confirming?.name ?? "" })
          }
          description={
            confirming?.kind === "revoke" ? t("revokeHelp") : t("rotateHelp")
          }
        >
          <form
            className="mcp-form mcp-form-single"
            noValidate
            onSubmit={confirmForm.handleSubmit(({ password }) => {
              if (!confirming) return;
              if (confirming.kind === "rotate")
                rotate.mutate({ id: confirming.id, password });
              else revoke.mutate({ id: confirming.id, password });
            })}
          >
            <div className="editor-field">
              <label htmlFor="mcp-confirm-password">{t("password")}</label>
              <Input
                id="mcp-confirm-password"
                type="password"
                autoComplete="current-password"
                aria-invalid={!!confirmForm.formState.errors.password}
                aria-describedby={
                  confirmForm.formState.errors.password
                    ? "mcp-confirm-password-error"
                    : undefined
                }
                {...confirmForm.register("password")}
              />
              <FieldError
                id="mcp-confirm-password-error"
                message={
                  confirmForm.formState.errors.password
                    ? t("validation.password")
                    : undefined
                }
              />
            </div>
            {(rotate.isError || revoke.isError) && (
              <p role="alert" className="form-error">
                {failureText(rotate.error ?? revoke.error, t)}
              </p>
            )}
            <div className="mcp-dialog-actions">
              <Button
                type="button"
                variant="outline"
                onClick={() => setConfirming(null)}
              >
                {t("cancel")}
              </Button>
              <Button
                type="submit"
                disabled={rotate.isPending || revoke.isPending}
              >
                {confirming?.kind === "revoke"
                  ? t("confirmRevoke")
                  : t("confirmRotate")}
              </Button>
            </div>
          </form>
        </EditorDialog>
        <EditorDialog
          open={emergencyOpen}
          onOpenChange={setEmergencyOpen}
          title={t("emergencyTitle")}
          description={t("emergencyHelp")}
        >
          <form
            className="mcp-form"
            noValidate
            onSubmit={adminForm.handleSubmit((value) =>
              adminRevoke.mutate(value),
            )}
          >
            <div className="editor-field">
              <label htmlFor="mcp-emergency-id">{t("emergencyId")}</label>
              <Input
                id="mcp-emergency-id"
                type="number"
                min={1}
                aria-invalid={!!adminForm.formState.errors.id}
                aria-describedby={
                  adminForm.formState.errors.id
                    ? "mcp-emergency-id-error"
                    : undefined
                }
                {...adminForm.register("id", { valueAsNumber: true })}
              />
              <FieldError
                id="mcp-emergency-id-error"
                message={
                  adminForm.formState.errors.id ? t("validation.id") : undefined
                }
              />
            </div>
            <div className="editor-field">
              <label htmlFor="mcp-emergency-password">
                {t("adminPassword")}
              </label>
              <Input
                id="mcp-emergency-password"
                type="password"
                autoComplete="current-password"
                aria-invalid={!!adminForm.formState.errors.password}
                aria-describedby={
                  adminForm.formState.errors.password
                    ? "mcp-emergency-password-error"
                    : undefined
                }
                {...adminForm.register("password")}
              />
              <FieldError
                id="mcp-emergency-password-error"
                message={
                  adminForm.formState.errors.password
                    ? t("validation.password")
                    : undefined
                }
              />
            </div>
            {adminRevoke.isError && (
              <p role="alert" className="form-error">
                {failureText(adminRevoke.error, t)}
              </p>
            )}
            <div className="mcp-dialog-actions">
              <Button
                type="button"
                variant="outline"
                onClick={() => setEmergencyOpen(false)}
              >
                {t("cancel")}
              </Button>
              <Button type="submit" disabled={adminRevoke.isPending}>
                {t("emergencySubmit")}
              </Button>
            </div>
          </form>
        </EditorDialog>
        {adminRevoke.isSuccess && (
          <p role="status" className="muted-text">
            {t("emergencyDone")}
          </p>
        )}
      </section>
      <section className="admin-panel webmcp-section">
        <h2>{t("connect")}</h2>
        <p>{t("import")}</p>
        <pre className="webmcp-code">{`[mcp_servers.baseline-business]\nurl = "${window.location.origin}/mcp"\nbearer_token_env_var = "BASELINE_MCP_TOKEN"`}</pre>
      </section>
      <section className="admin-panel webmcp-section">
        <h2>{t("workbuddy")}</h2>
        <p>{t("workbuddyHelp")}</p>
        <pre className="webmcp-code">
          {JSON.stringify(
            {
              mcpServers: {
                "baseline-business": {
                  type: "streamableHttp",
                  url: `${window.location.origin}/mcp`,
                  headers: { Authorization: "Bearer ${BASELINE_MCP_TOKEN}" },
                },
              },
            },
            null,
            2,
          )}
        </pre>
      </section>
      <section className="admin-panel webmcp-section">
        <h2>{t("helper")}</h2>
        <p>{t("helperHelp")}</p>
        <code className="webmcp-code mcp-helper-command">{`MCP_BUSINESS_URL=${window.location.origin}/mcp pnpm mcp:helper`}</code>
        <div className="mcp-form">
          <div className="editor-field">
            <label htmlFor="business-pairing">{t("pairing")}</label>
            <Input
              id="business-pairing"
              type="password"
              autoComplete="off"
              {...helperForm.register("key")}
            />
          </div>
          <div className="editor-field">
            <label htmlFor="business-token">{t("credential")}</label>
            <Input
              id="business-token"
              type="password"
              autoComplete="off"
              {...helperForm.register("token")}
            />
          </div>
        </div>
        <div className="webmcp-actions">
          <Button
            disabled={helperStatus === "busy"}
            onClick={() => void diagnose()}
          >
            {t("diagnose")}
          </Button>
          <Button
            disabled={helperStatus === "busy"}
            onClick={() => void prepare("codex")}
          >
            {t("installCodex")}
          </Button>
          <Button
            disabled={helperStatus === "busy"}
            onClick={() => void prepare("workbuddy")}
          >
            {t("installWorkbuddy")}
          </Button>
        </div>
        <EditorDialog
          open={!!prepared}
          onOpenChange={(open) => {
            if (!open) setPrepared(null);
          }}
          title={t("confirmInstall")}
          description={t("installTarget", {
            client: prepared?.client ?? "",
            url: prepared?.url ?? "",
          })}
        >
          <div className="webmcp-actions">
            <Button
              disabled={helperStatus === "busy"}
              onClick={() => void install()}
            >
              {t("confirmInstall")}
            </Button>
            <Button onClick={() => setPrepared(null)}>{t("cancel")}</Button>
          </div>
        </EditorDialog>
        <p role="status">{t(`helperStatus.${helperStatus}`)}</p>
      </section>
      <section className="admin-panel webmcp-section">
        <h2>{t("audit")}</h2>
        <Button onClick={() => void audit.refetch()}>{t("refresh")}</Button>
        {(audit.data?.status === 200 ? audit.data.data : []).map((item) => (
          <p key={item.id}>
            {t("auditEntry", {
              tool: item.tool,
              result: item.resultCode,
              time: new Intl.DateTimeFormat(i18n.resolvedLanguage, {
                dateStyle: "short",
                timeStyle: "medium",
              }).format(new Date(item.createdAt)),
            })}
          </p>
        ))}
      </section>
    </div>
  );
}
