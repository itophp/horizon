import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Info } from "lucide-react";
import { Checkbox } from "@/components/ui/checkbox";
import type {
  PermissionGrant,
  PermissionGrantScope,
  DepartmentRecord,
  PermissionDefinition,
} from "@/api/generated/models";
export function scopeOptions(t: TFunction) {
  return [
    "ALL",
    "ORGANIZATION",
    "DEPARTMENT",
    "DEPARTMENT_TREE",
    "OWN",
    "CUSTOM",
  ].map((value, index) => ({
    value: value as PermissionGrantScope,
    label: [
      t("admin:permissions.all"),
      t("admin:permissions.organization"),
      t("admin:permissions.department"),
      t("admin:permissions.departmentTree"),
      t("admin:permissions.own"),
      t("admin:permissions.custom"),
    ][index],
  }));
}
export function permissionFunctionLabel(key: string, t: TFunction) {
  const labels: Record<string, string> = {
    accounts: t("admin:nav.accounts"),
    organizations: t("admin:nav.organizations"),
    departments: t("admin:nav.departments"),
    roles: t("admin:nav.roles"),
    policies: t("admin:permissions.grantAdmin"),
    temporary: t("admin:temporary.approvalPermission"),
    settings: t("governance:settings"),
    dictionaries: t("governance:dictionaries"),
    files: t("governance:files"),
    notifications: t("governance:messages"),
    mcp: t("mcp:title"),
  };
  return labels[key] ?? key;
}
export function PermissionMatrix({
  grants,
  onChange,
  departments,
  catalog = [],
  account = false,
  disabled = false,
  request = false,
}: {
  grants: PermissionGrant[];
  onChange: (value: PermissionGrant[]) => void;
  departments: Pick<DepartmentRecord, "id" | "name">[];
  catalog?: PermissionDefinition[];
  account?: boolean;
  disabled?: boolean;
  request?: boolean;
}) {
  const { t } = useTranslation();
  const definitions = catalog.filter(
    (definition) => !request || definition.temporaryEligible,
  );
  const [scope, setScope] = useState<PermissionGrantScope>(
    grants.find(
      (g) => catalog.find((d) => d.code === g.permission)?.scopeConfigurable,
    )?.scope ?? "DEPARTMENT",
  );
  const [selectedDepartments, setDepartments] = useState<number[]>(
    grants.find((g) => g.scope === "CUSTOM")?.departmentIds ?? [],
  );
  const actions = [
    { key: "read", label: t("admin:permissions.read") },
    { key: "create", label: t("admin:permissions.create") },
    { key: "update", label: t("admin:permissions.update") },
    { key: "delete", label: t("admin:permissions.delete") },
  ];
  const additional = definitions.some(
    (d) =>
      !["read", "create", "update", "delete", "manage", "approve"].includes(
        d.action,
      ),
  );
  const modules = [...new Set(definitions.map((d) => d.module))].map((key) => ({
    key,
    functions: [
      ...new Set(
        definitions.filter((d) => d.module === key).map((d) => d.function),
      ),
    ],
  }));
  function changeScope(next: PermissionGrantScope) {
    setScope(next);
    onChange(
      grants.map((g) =>
        catalog.find((d) => d.code === g.permission)?.scopeConfigurable
          ? {
              ...g,
              scope: next,
              departmentIds: next === "CUSTOM" ? selectedDepartments : [],
            }
          : g,
      ),
    );
  }
  function changeDepartments(ids: number[]) {
    setDepartments(ids);
    onChange(
      grants.map((g) =>
        catalog.find((d) => d.code === g.permission)?.scopeConfigurable
          ? { ...g, departmentIds: scope === "CUSTOM" ? ids : [] }
          : g,
      ),
    );
  }
  function setGrant(
    definition: PermissionDefinition,
    effect: "allow" | "deny" | "inherit",
  ) {
    const next = grants.filter((g) => g.permission !== definition.code);
    if (effect !== "inherit")
      next.push({
        permission: definition.code,
        effect,
        scope: definition.scopeConfigurable ? scope : "ALL",
        departmentIds:
          definition.scopeConfigurable && scope === "CUSTOM"
            ? selectedDepartments
            : [],
      });
    onChange(next);
  }
  function control(definition: PermissionDefinition, single = false) {
    const effect =
      grants.find((g) => g.permission === definition.code)?.effect ?? "inherit";
    const label = single
      ? permissionFunctionLabel(definition.function, t)
      : `${permissionFunctionLabel(definition.function, t)} ${actions.find((a) => a.key === definition.action)?.label ?? t(`governance:actions.${definition.action}`)}`;
    return account ? (
      <div className="grant-switch" role="group" aria-label={label}>
        {(["inherit", "allow", "deny"] as const).map((value) => (
          <button
            key={value}
            type="button"
            disabled={disabled}
            className={effect === value ? `selected ${value}` : ""}
            aria-pressed={effect === value}
            onClick={() => setGrant(definition, value)}
          >
            {value === "inherit"
              ? t("admin:permissions.inherit")
              : value === "allow"
                ? t("admin:permissions.allow")
                : t("admin:permissions.deny")}
          </button>
        ))}
      </div>
    ) : (
      <Checkbox
        disabled={disabled}
        checked={effect === "allow"}
        aria-label={label}
        onCheckedChange={(checked) =>
          setGrant(definition, checked ? "allow" : "inherit")
        }
      />
    );
  }
  return (
    <div className="permission-editor">
      <div className="scope-editor">
        <div>
          <span className="field-label">
            {t("admin:permissions.dataScope")}
          </span>
          <p>{t("admin:permissions.scopeHint")}</p>
        </div>
        <select
          aria-label={t("admin:permissions.dataScope")}
          value={scope}
          disabled={disabled}
          onChange={(event) =>
            changeScope(event.target.value as PermissionGrantScope)
          }
        >
          {scopeOptions(t).map((option) => (
            <option value={option.value} key={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>
      {scope === "CUSTOM" && (
        <div className="custom-departments">
          <strong>{t("admin:permissions.customDepartments")}</strong>
          <div>
            {departments.map((department) => (
              <label key={department.id}>
                <Checkbox
                  disabled={disabled}
                  checked={selectedDepartments.includes(department.id)}
                  onCheckedChange={(checked) =>
                    changeDepartments(
                      checked
                        ? [...selectedDepartments, department.id]
                        : selectedDepartments.filter(
                            (id) => id !== department.id,
                          ),
                    )
                  }
                />
                {department.name}
              </label>
            ))}
          </div>
        </div>
      )}
      <div className="permission-matrix">
        <table className="permission-hierarchy">
          <thead>
            <tr>
              <th scope="col">{t("admin:permissions.module")}</th>
              <th scope="col">{t("admin:permissions.function")}</th>
              {actions.map((action) => (
                <th scope="col" key={action.key}>
                  {action.label}
                </th>
              ))}
              {additional && (
                <th scope="col">{t("governance:additionalActions")}</th>
              )}
            </tr>
          </thead>
          {modules.map((module) => (
            <tbody key={module.key}>
              {module.functions.map((fn, index) => {
                const entries = definitions.filter(
                  (d) => d.module === module.key && d.function === fn,
                );
                const single =
                  entries.length === 1 &&
                  ["manage", "approve"].includes(entries[0].action);
                return (
                  <tr key={fn}>
                    {index === 0 && (
                      <th
                        scope="rowgroup"
                        rowSpan={module.functions.length}
                        className="permission-module"
                      >
                        {module.key === "identity"
                          ? t("admin:permissions.identityModule")
                          : module.key === "access"
                            ? t("admin:permissions.accessModule")
                            : t("governance:systemModule")}
                      </th>
                    )}
                    <th scope="row">{permissionFunctionLabel(fn, t)}</th>
                    {single ? (
                      <td colSpan={additional ? 5 : 4}>
                        <div className="matrix-cell permission-sensitive">
                          {control(entries[0], true)}
                          <span>
                            {fn === "policies"
                              ? t("admin:permissions.grantAdminHint")
                              : t("admin:temporary.approvalHint")}
                          </span>
                        </div>
                      </td>
                    ) : (
                      actions.map((action) => {
                        const definition = entries.find(
                          (d) => d.action === action.key,
                        );
                        return (
                          <td key={action.key}>
                            <div className="matrix-cell">
                              {definition ? (
                                control(definition)
                              ) : (
                                <span
                                  className="muted-text"
                                  aria-label={t(
                                    "admin:permissions.notApplicable",
                                  )}
                                >
                                  {t("admin:permissions.notApplicable")}
                                </span>
                              )}
                            </div>
                          </td>
                        );
                      })
                    )}
                    {additional && !single && (
                      <td>
                        <div className="gov-extra-grants">
                          {entries
                            .filter(
                              (d) =>
                                ![
                                  "read",
                                  "create",
                                  "update",
                                  "delete",
                                ].includes(d.action),
                            )
                            .map((d) => (
                              <label key={d.code}>
                                {control(d)}
                                <span>
                                  {t(`governance:actions.${d.action}`)}
                                </span>
                              </label>
                            ))}
                        </div>
                      </td>
                    )}
                  </tr>
                );
              })}
            </tbody>
          ))}
        </table>
      </div>
      {account && (
        <div className="permission-legend">
          <Info size={15} />
          {t("admin:permissions.legend")}
        </div>
      )}
    </div>
  );
}
