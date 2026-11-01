import { useConfiguration } from "@/features/governance/runtime";
import { useMutation } from "@tanstack/react-query";
import {
  identityCreateAccount,
  identityCreateDepartment,
  identityCreateOrganization,
  identityCreateRole,
  identityUpdateAccount,
  identityUpdateDepartment,
  identityUpdateOrganization,
  identityUpdateRole,
} from "@/lib/capabilities";
import { useState, useId, cloneElement, type ReactElement } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import { LoaderCircle } from "lucide-react";
import type {
  AccountRecord,
  OrganizationRecord,
  DepartmentRecord,
  RoleRecord,
} from "@/api/generated/models";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { useSession } from "./auth";
import {
  ErrorBanner,
  useAdminData,
  useRefreshAdmin,
  validationText,
  ScopeLabel,
} from "./shared";
import { PermissionMatrix } from "./permission-matrix";
// The console no longer owns per-resource REST hooks: each form mutation is one
// capability invocation, so the gateway records the same audit as an agent call.
const useIdentityCreateOrganization = () =>
  useMutation({
    mutationFn: ({ data }: { data: unknown }) =>
      identityCreateOrganization(data),
  });
const useIdentityUpdateOrganization = () =>
  useMutation({
    mutationFn: ({ id, data }: { id: number; data: unknown }) =>
      identityUpdateOrganization(id, data),
  });
const useIdentityCreateDepartment = () =>
  useMutation({
    mutationFn: ({ data }: { data: unknown }) => identityCreateDepartment(data),
  });
const useIdentityUpdateDepartment = () =>
  useMutation({
    mutationFn: ({ id, data }: { id: number; data: unknown }) =>
      identityUpdateDepartment(id, data),
  });
const useIdentityCreateRole = () =>
  useMutation({
    mutationFn: ({ data }: { data: unknown }) => identityCreateRole(data),
  });
const useIdentityUpdateRole = () =>
  useMutation({
    mutationFn: ({ id, data }: { id: number; data: unknown }) =>
      identityUpdateRole(id, data),
  });
const useIdentityCreateAccount = () =>
  useMutation({
    mutationFn: ({ data }: { data: unknown }) => identityCreateAccount(data),
  });
const useIdentityUpdateAccount = () =>
  useMutation({
    mutationFn: ({ id, data }: { id: number; data: unknown }) =>
      identityUpdateAccount(id, data),
  });

const required = z
  .string()
  .trim()
  .min(1, { message: "required" })
  .max(100, { message: "required" });
const code = z.string().regex(/^[A-Za-z0-9_-]{2,40}$/, { message: "code" });
const status = z.enum(["active", "inactive"]);
const grantSchema = z
  .object({
    permission: z.string(),
    effect: z.enum(["allow", "deny"]),
    scope: z.enum([
      "ALL",
      "ORGANIZATION",
      "DEPARTMENT",
      "DEPARTMENT_TREE",
      "OWN",
      "CUSTOM",
    ]),
    departmentIds: z.array(z.number()),
  })
  .refine(
    (value) => value.scope !== "CUSTOM" || value.departmentIds.length > 0,
    { message: "departments" },
  );
const organizationSchema = z.object({
  name: required,
  code,
  kind: z.enum(["enterprise", "company"]),
  parentId: z.number(),
  status,
  description: z.string().max(500),
});
const departmentSchema = z.object({
  name: required,
  code,
  organizationId: z.number().min(1, { message: "required" }),
  parentId: z.number(),
  manager: z.string().max(100),
  status,
  description: z.string().max(500),
});
const roleSchema = z.object({
  name: required,
  code,
  description: z.string().max(500),
  status,
  grants: z.array(grantSchema),
});
const membershipSchema = z.object({
  organizationId: z.number().min(1),
  departmentId: z.number().min(1),
  roleIds: z.array(z.number()),
  overrides: z.array(grantSchema),
});
const accountSchema = z.object({
  additionalMemberships: z.array(membershipSchema).max(49),
  username: z
    .string()
    .regex(/^[A-Za-z0-9_.@-]{3,80}$/, { message: "username" }),
  fullName: required,
  email: z
    .string()
    .refine((value) => value === "" || z.email().safeParse(value).success, {
      message: "email",
    }),
  status,
  organizationId: z.number().min(1, { message: "required" }),
  departmentId: z.number().min(1, { message: "required" }),
  roleIds: z.array(z.number()),
  overrides: z.array(grantSchema),
  password: z
    .string()
    .refine(
      (value) => value === "" || (value.length >= 12 && value.length <= 128),
      { message: "password" },
    ),
});
export function FormField({
  label,
  error,
  children,
  wide = false,
}: {
  label: string;
  error?: string;
  children: ReactElement<{
    id?: string;
    "aria-invalid"?: boolean;
    "aria-describedby"?: string;
  }>;
  wide?: boolean;
}) {
  const { t } = useTranslation();
  const config = useConfiguration();
  const generated = useId();
  const id = children.props.id ?? generated;
  return (
    <div className={`editor-field ${wide ? "field-wide" : ""}`}>
      <label htmlFor={id}>{label}</label>
      {cloneElement(children, {
        id,
        "aria-invalid": !!error,
        "aria-describedby": error ? `${id}-error` : undefined,
      })}
      {error && (
        <small id={`${id}-error`} className="field-error" role="alert">
          {error === "passwordPolicy" || error === "password"
            ? t("governance:passwordPolicy", {
                count: Number(config.value("security.passwordMin", 12)),
              })
            : validationText(error, t)}
        </small>
      )}
    </div>
  );
}

function FormActions({
  busy,
  onCancel,
  disabled = false,
}: {
  busy: boolean;
  onCancel?: () => void;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <div className="editor-footer">
      {onCancel && (
        <Button type="button" variant="outline" onClick={onCancel}>
          {t("admin:actions.cancel")}
        </Button>
      )}
      <Button type="submit" disabled={busy || disabled}>
        {busy && <LoaderCircle size={14} className="animate-spin" />}
        {busy ? t("admin:actions.saving") : t("admin:actions.save")}
      </Button>
    </div>
  );
}
export function OrganizationForm({
  record,
  onSaved,
  onCancel,
  kind = "enterprise",
}: {
  record?: OrganizationRecord;
  onSaved: () => void;
  onCancel?: () => void;
  kind?: "enterprise" | "company";
}) {
  const { t } = useTranslation();
  const { data } = useAdminData();
  const refresh = useRefreshAdmin();
  const [error, setError] = useState<unknown>(null);
  const create = useIdentityCreateOrganization();
  const update = useIdentityUpdateOrganization();
  const form = useForm({
    resolver: zodResolver(organizationSchema),
    defaultValues: {
      name: record?.name ?? "",
      code: record?.code ?? "",
      kind: (record?.kind as "enterprise" | "company") ?? kind,
      parentId: record?.parentId ?? 0,
      status: (record?.status as "active" | "inactive") ?? "active",
      description: record?.description ?? "",
    },
  });
  const selectedKind = useWatch({ control: form.control, name: "kind" });
  const busy = create.isPending || update.isPending;
  const submit = form.handleSubmit(async (values) => {
    setError(null);
    try {
      const payload = {
        ...values,
        parentId: values.kind === "enterprise" ? 0 : values.parentId,
      };
      if (record) await update.mutateAsync({ id: record.id, data: payload });
      else await create.mutateAsync({ data: payload });
      await refresh();
      onSaved();
    } catch (cause) {
      setError(cause);
    }
  });
  return (
    <form onSubmit={submit} noValidate className="entity-form">
      {!!error && <ErrorBanner error={error} />}
      <div className="editor-grid">
        <FormField
          label={t("admin:fields.name")}
          error={form.formState.errors.name?.message}
        >
          <Input {...form.register("name")} />
        </FormField>
        <FormField
          label={t("admin:fields.code")}
          error={form.formState.errors.code?.message}
        >
          <Input {...form.register("code")} />
        </FormField>
        <FormField label={t("admin:organizations.kind")}>
          <select {...form.register("kind")} disabled={!!record}>
            <option value="enterprise">
              {t("admin:organizations.enterprise")}
            </option>
            <option value="company">{t("admin:organizations.company")}</option>
          </select>
        </FormField>
        <FormField label={t("admin:fields.status")}>
          <select {...form.register("status")}>
            <option value="active">{t("admin:status.active")}</option>
            <option value="inactive">{t("admin:status.inactive")}</option>
          </select>
        </FormField>
        {selectedKind === "company" && (
          <FormField label={t("admin:organizations.parent")} wide>
            <select {...form.register("parentId", { valueAsNumber: true })}>
              <option value={0}>{t("admin:fields.select")}</option>
              {data.organizations
                .filter(
                  (org) => org.kind === "enterprise" && org.status === "active",
                )
                .map((org) => (
                  <option key={org.id} value={org.id}>
                    {org.name}
                  </option>
                ))}
            </select>
          </FormField>
        )}
        <FormField label={t("admin:fields.description")} wide>
          <textarea rows={3} {...form.register("description")} />
        </FormField>
      </div>
      {record && (
        <p className="form-hint">{t("admin:organizations.kindHint")}</p>
      )}
      <FormActions busy={busy} onCancel={onCancel} />
    </form>
  );
}
export function DepartmentForm({
  record,
  onSaved,
  onCancel,
  companyId = 0,
}: {
  record?: DepartmentRecord;
  onSaved: () => void;
  onCancel?: () => void;
  companyId?: number;
}) {
  const { t } = useTranslation();
  const { data } = useAdminData();
  const refresh = useRefreshAdmin();
  const [error, setError] = useState<unknown>(null);
  const create = useIdentityCreateDepartment();
  const update = useIdentityUpdateDepartment();
  const form = useForm({
    resolver: zodResolver(departmentSchema),
    defaultValues: {
      name: record?.name ?? "",
      code: record?.code ?? "",
      organizationId: record?.organizationId ?? companyId,
      parentId: record?.parentId ?? 0,
      manager: record?.manager ?? "",
      status: (record?.status as "active" | "inactive") ?? "active",
      description: record?.description ?? "",
    },
  });
  const organization = useWatch({
    control: form.control,
    name: "organizationId",
  });
  const busy = create.isPending || update.isPending;
  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        setError(null);
        try {
          if (record) await update.mutateAsync({ id: record.id, data: values });
          else await create.mutateAsync({ data: values });
          await refresh();
          onSaved();
        } catch (cause) {
          setError(cause);
        }
      })}
    >
      {!!error && <ErrorBanner error={error} />}
      <div className="editor-grid">
        <FormField
          label={t("admin:fields.name")}
          error={form.formState.errors.name?.message}
        >
          <Input {...form.register("name")} />
        </FormField>
        <FormField
          label={t("admin:fields.code")}
          error={form.formState.errors.code?.message}
        >
          <Input {...form.register("code")} />
        </FormField>
        <FormField
          label={t("admin:fields.organization")}
          error={form.formState.errors.organizationId?.message}
        >
          <select
            {...form.register("organizationId", { valueAsNumber: true })}
            disabled={!!record}
          >
            <option value={0}>{t("admin:fields.select")}</option>
            {data.organizations
              .filter(
                (org) => org.kind === "company" && org.status === "active",
              )
              .map((org) => (
                <option value={org.id} key={org.id}>
                  {org.name}
                </option>
              ))}
          </select>
        </FormField>
        <FormField label={t("admin:departments.parent")}>
          <select {...form.register("parentId", { valueAsNumber: true })}>
            <option value={0}>{t("admin:departments.root")}</option>
            {data.departments
              .filter(
                (dept) =>
                  dept.organizationId === organization &&
                  dept.id !== record?.id,
              )
              .map((dept) => (
                <option value={dept.id} key={dept.id}>
                  {dept.name}
                </option>
              ))}
          </select>
        </FormField>
        <FormField label={t("admin:departments.manager")}>
          <Input {...form.register("manager")} />
        </FormField>
        <FormField label={t("admin:fields.status")}>
          <select {...form.register("status")}>
            <option value="active">{t("admin:status.active")}</option>
            <option value="inactive">{t("admin:status.inactive")}</option>
          </select>
        </FormField>
        <FormField label={t("admin:fields.description")} wide>
          <textarea rows={3} {...form.register("description")} />
        </FormField>
      </div>
      <FormActions busy={busy} onCancel={onCancel} />
    </form>
  );
}
export function RoleForm({
  record,
  onSaved,
  onCancel,
  permissionsOnly = false,
}: {
  record?: RoleRecord;
  onSaved: () => void;
  onCancel?: () => void;
  permissionsOnly?: boolean;
}) {
  const { t } = useTranslation();
  const { data } = useAdminData();
  const session = useSession();
  const editable =
    session.permissions.includes("access:manage") &&
    session.permissions.includes(record ? "roles:update" : "roles:create");
  const refresh = useRefreshAdmin();
  const [error, setError] = useState<unknown>(null);
  const create = useIdentityCreateRole();
  const update = useIdentityUpdateRole();
  const form = useForm({
    resolver: zodResolver(roleSchema),
    defaultValues: {
      name: record?.name ?? "",
      code: record?.code ?? "",
      status: (record?.status as "active" | "inactive") ?? "active",
      description: record?.description ?? "",
      grants: record?.grants ?? [],
    },
  });
  const grants = useWatch({ control: form.control, name: "grants" });
  const busy = create.isPending || update.isPending;
  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        setError(null);
        try {
          if (record) await update.mutateAsync({ id: record.id, data: values });
          else await create.mutateAsync({ data: values });
          await refresh();
          onSaved();
        } catch (cause) {
          setError(cause);
        }
      })}
    >
      {!!error && <ErrorBanner error={error} />}
      <fieldset disabled={!editable}>
        {!permissionsOnly && (
          <div className="editor-grid">
            <FormField
              label={t("admin:fields.name")}
              error={form.formState.errors.name?.message}
            >
              <Input {...form.register("name")} />
            </FormField>
            <FormField
              label={t("admin:fields.code")}
              error={form.formState.errors.code?.message}
            >
              <Input {...form.register("code")} />
            </FormField>
            <FormField label={t("admin:fields.status")}>
              <select {...form.register("status")}>
                <option value="active">{t("admin:status.active")}</option>
                <option value="inactive">{t("admin:status.inactive")}</option>
              </select>
            </FormField>
            <FormField label={t("admin:fields.description")} wide>
              <textarea rows={2} {...form.register("description")} />
            </FormField>
          </div>
        )}
        <div className="editor-section-heading">
          <h3>{t("admin:permissions.matrix")}</h3>
        </div>
        <PermissionMatrix
          catalog={data.catalog}
          grants={grants}
          departments={data.departments}
          disabled={!editable}
          onChange={(value) =>
            form.setValue("grants", value, {
              shouldValidate: true,
              shouldDirty: true,
            })
          }
        />
        {form.formState.errors.grants && (
          <p className="field-error" role="alert">
            {t("admin:validation.departments")}
          </p>
        )}
      </fieldset>
      {!editable && (
        <p className="form-hint">{t("admin:permissions.readOnly")}</p>
      )}
      <FormActions busy={busy} disabled={!editable} onCancel={onCancel} />
    </form>
  );
}
export function AccountForm({
  record,
  onSaved,
  onCancel,
  permissionsOnly = false,
  readOnly = false,
}: {
  record?: AccountRecord;
  onSaved: () => void;
  onCancel?: () => void;
  permissionsOnly?: boolean;
  readOnly?: boolean;
}) {
  const config = useConfiguration();
  const { t } = useTranslation();
  const { data } = useAdminData();
  const session = useSession();
  const refresh = useRefreshAdmin();
  const [error, setError] = useState<unknown>(null);
  const create = useIdentityCreateAccount();
  const update = useIdentityUpdateAccount();
  const [tab, setTab] = useState<"basics" | "access">(
    permissionsOnly ? "access" : "basics",
  );
  const form = useForm({
    resolver: zodResolver(accountSchema),
    defaultValues: {
      username: record?.username ?? "",
      fullName: record?.fullName ?? "",
      email: record?.email ?? "",
      status:
        (record?.status as "active" | "inactive") ??
        (config.value("security.initialStatus", "active") as
          "active" | "inactive"),
      organizationId: record?.organizationId ?? 0,
      departmentId: record?.departmentId ?? 0,
      roleIds: record?.roleIds ?? [],
      overrides: record?.overrides ?? [],
      password: "",
      additionalMemberships: (record?.memberships ?? [])
        .filter((m) => m.departmentId !== record?.departmentId)
        .map((m) => ({
          organizationId: m.organizationId,
          departmentId: m.departmentId,
          roleIds: m.roleIds,
          overrides: m.overrides,
        })),
    },
  });
  const [
    organization,
    primaryDepartment,
    primaryRoles,
    primaryOverrides,
    additional,
  ] = useWatch({
    control: form.control,
    name: [
      "organizationId",
      "departmentId",
      "roleIds",
      "overrides",
      "additionalMemberships",
    ],
  });
  const [membershipIndex, setMembershipIndex] = useState(-1);
  const [additionalDepartment, setAdditionalDepartment] = useState(0);
  const selectedMembership = additional[membershipIndex];
  const canManageMemberships =
    !readOnly &&
    session.permissions.includes("access:manage") &&
    (!record?.isAdmin || record.id === session.account.id);
  const canGrant =
    canManageMemberships && (!record?.isAdmin || !!selectedMembership);

  const roleIds = selectedMembership?.roleIds ?? primaryRoles;
  const overrides = selectedMembership?.overrides ?? primaryOverrides;
  const rolePath = selectedMembership
    ? (`additionalMemberships.${membershipIndex}.roleIds` as const)
    : "roleIds";
  const overridePath = selectedMembership
    ? (`additionalMemberships.${membershipIndex}.overrides` as const)
    : "overrides";
  const busy = create.isPending || update.isPending;
  return (
    <form
      className="entity-form"
      noValidate
      onSubmit={form.handleSubmit(async (values) => {
        setError(null);
        if (!record && !values.password) {
          form.setError("password", { message: "password" });
          setTab("basics");
          return;
        }
        if (
          values.password &&
          Array.from(values.password).length <
            Number(config.value("security.passwordMin", 12))
        ) {
          form.setError("password", { message: "passwordPolicy" });
          setTab("basics");
          return;
        }
        try {
          const { additionalMemberships, ...fields } = values;
          const payload = {
            ...fields,
            departmentAccess: { memberships: additionalMemberships },
          };
          if (record)
            await update.mutateAsync({ id: record.id, data: payload });
          else await create.mutateAsync({ data: payload });
          await refresh();
          onSaved();
        } catch (cause) {
          setError(cause);
        }
      })}
    >
      {!!error && <ErrorBanner error={error} />}
      {!permissionsOnly && (
        <div className="editor-tabs">
          <button
            type="button"
            aria-pressed={tab === "basics"}
            className={tab === "basics" ? "active" : ""}
            onClick={() => setTab("basics")}
          >
            {t("admin:accounts.basics")}
          </button>
          <button
            type="button"
            aria-pressed={tab === "access"}
            className={tab === "access" ? "active" : ""}
            onClick={() => setTab("access")}
          >
            {t("admin:memberships.tab")}
          </button>
        </div>
      )}
      <section
        className="membership-editor"
        aria-label={t("admin:memberships.title")}
      >
        <div className="editor-section-heading">
          <h3>{t("admin:memberships.title")}</h3>
          <p>{t("admin:memberships.hint")}</p>
        </div>
        <div className="cell-tags">
          <button
            type="button"
            className="neutral-badge"
            aria-pressed={membershipIndex === -1}
            onClick={() => {
              setMembershipIndex(-1);
              setTab("access");
            }}
          >
            {t("admin:memberships.primaryLabel", {
              department:
                data.departments.find((d) => d.id === primaryDepartment)
                  ?.name ?? t("admin:fields.unassigned"),
            })}
          </button>
          {additional.map((m, index) => (
            <button
              type="button"
              className="neutral-badge"
              key={m.departmentId}
              aria-pressed={membershipIndex === index}
              onClick={() => {
                setMembershipIndex(index);
                setTab("access");
              }}
            >
              {data.departments.find((d) => d.id === m.departmentId)?.name ??
                t("admin:fields.unassigned")}
            </button>
          ))}
        </div>
        {canManageMemberships && (
          <div className="membership-actions">
            <FormField label={t("admin:memberships.additional")}>
              <select
                value={additionalDepartment}
                onChange={(e) =>
                  setAdditionalDepartment(Number(e.target.value))
                }
              >
                <option value={0}>{t("admin:fields.select")}</option>
                {data.departments
                  .filter(
                    (d) =>
                      d.status === "active" &&
                      d.id !== primaryDepartment &&
                      !additional.some((m) => m.departmentId === d.id),
                  )
                  .map((d) => (
                    <option value={d.id} key={d.id}>
                      {t("admin:memberships.option", {
                        organization:
                          data.organizations.find(
                            (o) => o.id === d.organizationId,
                          )?.name ?? t("admin:fields.unassigned"),
                        department: d.name,
                      })}
                    </option>
                  ))}
              </select>
            </FormField>
            <Button
              type="button"
              variant="outline"
              disabled={!additionalDepartment || additional.length >= 49}
              onClick={() => {
                const dept = data.departments.find(
                  (d) => d.id === additionalDepartment,
                );
                if (!dept) return;
                form.setValue(
                  "additionalMemberships",
                  [
                    ...additional,
                    {
                      organizationId: dept.organizationId,
                      departmentId: dept.id,
                      roleIds: [],
                      overrides: [],
                    },
                  ],
                  { shouldDirty: true },
                );
                setMembershipIndex(additional.length);
                setAdditionalDepartment(0);
                setTab("access");
              }}
            >
              {t("admin:memberships.add")}
            </Button>
            {selectedMembership && (
              <Button
                type="button"
                variant="outline"
                onClick={() => {
                  form.setValue(
                    "additionalMemberships",
                    additional.filter((_, index) => index !== membershipIndex),
                    { shouldDirty: true },
                  );
                  setMembershipIndex(-1);
                }}
              >
                {t("admin:memberships.remove")}
              </Button>
            )}
          </div>
        )}
        {!readOnly && !canManageMemberships && (
          <p className="form-hint">{t("admin:memberships.restricted")}</p>
        )}
      </section>
      <fieldset disabled={readOnly}>
        {tab === "basics" ? (
          <div className="editor-grid">
            <FormField
              label={t("admin:fields.fullName")}
              error={form.formState.errors.fullName?.message}
            >
              <Input {...form.register("fullName")} />
            </FormField>
            <FormField
              label={t("admin:fields.username")}
              error={form.formState.errors.username?.message}
            >
              <Input autoComplete="off" {...form.register("username")} />
            </FormField>
            <FormField
              label={t("admin:fields.email")}
              error={form.formState.errors.email?.message}
            >
              <Input type="email" {...form.register("email")} />
            </FormField>
            <FormField label={t("admin:fields.status")}>
              <select {...form.register("status")} disabled={record?.isAdmin}>
                <option value="active">{t("admin:status.active")}</option>
                <option value="inactive">{t("admin:status.inactive")}</option>
              </select>
            </FormField>
            <FormField
              label={t("admin:fields.organization")}
              error={form.formState.errors.organizationId?.message}
            >
              <select
                {...form.register("organizationId", { valueAsNumber: true })}
                onChange={(event) => {
                  form.setValue("organizationId", Number(event.target.value));
                  form.setValue("departmentId", 0);
                  setMembershipIndex(-1);
                }}
              >
                <option value={0}>{t("admin:fields.select")}</option>
                {data.organizations
                  .filter(
                    (org) => org.kind === "company" && org.status === "active",
                  )
                  .map((org) => (
                    <option key={org.id} value={org.id}>
                      {org.name}
                    </option>
                  ))}
              </select>
            </FormField>
            <FormField
              label={t("admin:fields.department")}
              error={form.formState.errors.departmentId?.message}
            >
              <select
                {...form.register("departmentId", { valueAsNumber: true })}
              >
                <option value={0}>{t("admin:fields.select")}</option>
                {data.departments
                  .filter(
                    (dept) =>
                      dept.organizationId === organization &&
                      dept.status === "active" &&
                      !additional.some((m) => m.departmentId === dept.id),
                  )
                  .map((dept) => (
                    <option key={dept.id} value={dept.id}>
                      {dept.name}
                    </option>
                  ))}
              </select>
            </FormField>
            <FormField
              label={
                record
                  ? t("admin:accounts.resetPassword")
                  : t("admin:accounts.newPassword")
              }
              error={form.formState.errors.password?.message}
              wide
            >
              <Input
                type="password"
                autoComplete="new-password"
                {...form.register("password")}
              />
            </FormField>
            <p className="form-hint field-wide">
              {t("governance:passwordPolicy", {
                count: Number(config.value("security.passwordMin", 12)),
              })}
            </p>
          </div>
        ) : (
          <>
            <div className="editor-section-heading">
              <h3>{t("admin:accounts.roles")}</h3>
              <p>{t("admin:accounts.roleHint")}</p>
            </div>
            <div className="role-options">
              {data.roles.map((role) => (
                <label key={role.id}>
                  <Checkbox
                    disabled={!canGrant}
                    checked={roleIds.includes(role.id)}
                    onCheckedChange={(checked) =>
                      form.setValue(
                        rolePath,
                        checked
                          ? [...roleIds, role.id]
                          : roleIds.filter((id) => id !== role.id),
                        { shouldDirty: true },
                      )
                    }
                  />
                  <span className="role-option-content">
                    <strong>{role.name}</strong>
                    <small>
                      {t("admin:roles.permissions", {
                        count: role.grants.length,
                      })}
                    </small>
                  </span>
                  <ScopeLabel scope={role.grants[0]?.scope ?? "DEPARTMENT"} />
                </label>
              ))}
            </div>
            <div className="editor-section-heading">
              <h3>{t("admin:accounts.overrides")}</h3>
              <p>{t("admin:accounts.overrideHint")}</p>
            </div>
            {record?.isAdmin && !selectedMembership && (
              <p className="form-hint admin-protection">
                {t("admin:memberships.adminHint")}
              </p>
            )}
            <PermissionMatrix
              key={selectedMembership?.departmentId ?? primaryDepartment}
              catalog={data.catalog}
              grants={overrides}
              departments={data.departments}
              account
              disabled={!canGrant}
              onChange={(value) =>
                form.setValue(overridePath, value, {
                  shouldDirty: true,
                  shouldValidate: true,
                })
              }
            />
            {(form.formState.errors.overrides ||
              form.formState.errors.additionalMemberships?.[membershipIndex]
                ?.overrides) && (
              <p className="field-error" role="alert">
                {t("admin:validation.departments")}
              </p>
            )}
          </>
        )}
      </fieldset>
      {!readOnly && <FormActions busy={busy} onCancel={onCancel} />}
    </form>
  );
}
