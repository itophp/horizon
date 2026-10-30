import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useForm, useFieldArray, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslation } from "react-i18next";
import { Plus, ArrowLeft, Download, Upload, Trash2 } from "lucide-react";
import {
  dictionariesList,
  dictionariesGet,
  dictionariesDraft,
  dictionariesExport,
  dictionariesImport,
  dictionariesImportDraft,
  dictionariesRemove,
  dictionariesOptions,
} from "@/api/generated/baseline";
import type {
  ConfigTarget,
  DictionaryDefinition,
  DictionaryDocument,
  DictionaryItem,
  DictionaryDetail,
  ImportPreview,
} from "@/api/generated/models";
import { useSession } from "@/features/administration/auth";
import {
  useAdminData,
  EditorDialog,
  ErrorBanner,
  Loading,
  DataTable,
  type AdminColumn,
} from "@/features/administration/shared";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  ChangeHistory,
  NoAccess,
  useGovKey,
  useGovTask,
  downloadContent,
} from "./shared";
export function DictionarySection() {
  const { t, i18n } = useTranslation("governance");
  const session = useSession();
  const target = { kind: "workspace" as const, id: 0 };
  const [selected, setSelected] = useState<number | null>(null);
  const [create, setCreate] = useState(false);
  const task = useGovTask();
  const q = useQuery({
    queryKey: useGovKey("dictionaries", target.kind, target.id),
    queryFn: () =>
      dictionariesList({ targetKind: target.kind, targetId: target.id }),
    enabled: session.permissions.includes("dictionaries:read"),
    retry: false,
  });
  const columns: AdminColumn<DictionaryDefinition>[] = [
    {
      id: "name",
      header: t("dictionaries"),
      cell: (r) => (
        <button className="entity-link" onClick={() => setSelected(r.id)}>
          <strong>
            {i18n.resolvedLanguage === "zh-CN" && r.nameZh
              ? r.nameZh
              : r.nameEn}
          </strong>
          <small>{r.code}</small>
        </button>
      ),
    },
    { id: "category", header: t("category"), cell: (r) => r.category },
    {
      id: "kind",
      header: t("kind"),
      cell: (r) => <span className="scope-badge">{t(r.kind)}</span>,
    },
    { id: "status", header: t("status"), cell: (r) => t(r.status) },
    { id: "revision", header: t("history"), cell: (r) => r.revision },
    {
      id: "actions",
      header: t("select"),
      cell: (r) => (
        <div className="heading-actions">
          <Button variant="outline" onClick={() => setSelected(r.id)}>
            {t("edit")}
          </Button>
          {r.revision === 0 &&
            session.permissions.includes("dictionaries:delete") && (
              <Button
                variant="ghost"
                aria-label={t("remove")}
                onClick={() => task.mutate(() => dictionariesRemove(r.id))}
              >
                <Trash2 size={15} />
              </Button>
            )}
        </div>
      ),
    },
  ];
  if (!session.permissions.includes("dictionaries:read")) return <NoAccess />;
  return (
    <>
      <div className="standalone-toolbar">
        <p className="muted-text">{t("dictionariesHint")}</p>
        <div className="toolbar-right">
          {selected !== null ? (
            <Button variant="outline" onClick={() => setSelected(null)}>
              <ArrowLeft size={15} />
              {t("dictionaries")}
            </Button>
          ) : (
            session.permissions.includes("dictionaries:create") && (
              <Button onClick={() => setCreate(true)}>
                <Plus size={16} />
                {t("create")}
              </Button>
            )
          )}
        </div>
      </div>
      {task.error && <ErrorBanner error={task.error} />}{" "}
      {selected !== null ? (
        <DictionaryEditor id={selected} target={target} />
      ) : q.isPending ? (
        <Loading />
      ) : q.error ? (
        <ErrorBanner error={q.error} />
      ) : (
        <section className="admin-panel entity-card">
          <DataTable
            rows={q.data?.status === 200 ? q.data.data.definitions : []}
            columns={columns}
          />
        </section>
      )}
      {create && (
        <EditorDialog
          open
          wide
          onOpenChange={setCreate}
          title={t("create")}
          description={t("dictionariesHint")}
        >
          <DictionaryForm
            target={target}
            onSaved={(id) => {
              setCreate(false);
              setSelected(id);
            }}
          />
        </EditorDialog>
      )}
    </>
  );
}
function DictionaryEditor({
  id,
  target,
}: {
  id: number;
  target: ConfigTarget;
}) {
  const { t } = useTranslation("governance");
  const session = useSession();
  const task = useGovTask();
  const [importing, setImporting] = useState(false);
  const q = useQuery({
    queryKey: useGovKey("dictionary", id, target.kind, target.id),
    queryFn: () =>
      dictionariesGet(id, { targetKind: "workspace", targetId: 0 }),
    retry: false,
  });
  if (q.isPending) return <Loading />;
  if (q.error) return <ErrorBanner error={q.error} />;
  const detail = q.data?.status === 200 ? q.data.data : undefined;
  if (!detail) return null;
  const own =
    detail.document.definition.target.kind === target.kind &&
    detail.document.definition.target.id === target.id;
  return (
    <>
      {own && (
        <div className="gov-toolbar">
          <span className="muted-text">
            {t("references", { count: detail.references })}
          </span>
          <div className="heading-actions">
            {session.permissions.includes("dictionaries:import") &&
              detail.document.definition.kind !== "dynamic" && (
                <Button variant="outline" onClick={() => setImporting(true)}>
                  <Upload size={15} />
                  {t("import")}
                </Button>
              )}
            {session.permissions.includes("dictionaries:export") &&
              detail.document.definition.kind !== "dynamic" &&
              ["csv", "json"].map((format) => (
                <Button
                  key={format}
                  variant="outline"
                  disabled={task.isPending}
                  onClick={() =>
                    task.mutate(async () => {
                      const r = await dictionariesExport(id, {
                        format: format as "csv" | "json",
                      });
                      if (r.status === 200)
                        downloadContent(
                          r.data.filename,
                          r.data.content,
                          format === "csv"
                            ? "text/csv;charset=utf-8"
                            : "application/json",
                        );
                    })
                  }
                >
                  <Download size={15} />
                  {t("exportFormat", { format: format.toUpperCase() })}
                </Button>
              ))}
          </div>
        </div>
      )}
      {task.error && <ErrorBanner error={task.error} />}
      <DictionaryForm
        key={`${target.kind}:${target.id}:${detail.document.definition.revision}:${detail.overrideRevision}`}
        target={target}
        detail={detail}
      />
      <DictionaryOptions code={detail.document.definition.code} />
      <ChangeHistory
        drafts={detail.drafts}
        history={detail.history}
        domain="dictionaries"
        revision={detail.document.definition.revision}
        overrideRevision={detail.overrideRevision}
      />
      {importing && (
        <ImportDialog
          document={detail.document}
          onClose={() => setImporting(false)}
        />
      )}
    </>
  );
}
const emptyItem: DictionaryItem = {
  id: 0,
  code: "",
  labelEn: "",
  labelZh: "",
  descriptionEn: "",
  descriptionZh: "",
  parentCode: "",
  sort: 0,
  status: "active",
  isDefault: false,
  startsAt: "",
  endsAt: "",
  color: "",
  icon: "",
};
const itemSchema = z.object({
  id: z.number(),
  code: z.string().regex(/^[A-Za-z0-9_.-]{1,80}$/),
  labelEn: z.string().trim().min(1).max(200),
  labelZh: z.string().max(200),
  descriptionEn: z.string(),
  descriptionZh: z.string(),
  parentCode: z.string(),
  sort: z.number(),
  status: z.enum(["active", "inactive", "archived"]),
  isDefault: z.boolean(),
  startsAt: z.string(),
  endsAt: z.string(),
  color: z.string(),
  icon: z.string(),
});
const dictionarySchema = z.object({
  code: z.string().regex(/^[A-Za-z0-9_.-]{1,80}$/),
  nameEn: z.string().trim().min(1),
  nameZh: z.string(),
  category: z.string().min(1),
  description: z.string(),
  kind: z.enum(["flat", "tree", "dynamic"]),
  provider: z.enum(["", "organizations", "departments", "roles", "accounts"]),
  status: z.enum(["active", "inactive", "archived"]),
  visibility: z.enum(["shared", "management"]),
  consumerPermission: z.string(),
  items: z.array(itemSchema).max(10000),
  reason: z.string().trim().min(5),
});
function DictionaryForm({
  target,
  detail,
  onSaved,
}: {
  target: ConfigTarget;
  detail?: DictionaryDetail;
  onSaved?: (id: number) => void;
}) {
  const { t } = useTranslation("governance");
  const session = useSession();
  const { data: admin } = useAdminData();
  const task = useGovTask();
  const original = detail?.document;
  const d = original?.definition;
  const writable = session.permissions.includes(
    d ? "dictionaries:update" : "dictionaries:create",
  );
  const form = useForm({
    resolver: zodResolver(dictionarySchema),
    defaultValues: {
      code: d?.code ?? "",
      nameEn: d?.nameEn ?? "",
      nameZh: d?.nameZh ?? "",
      category: d?.category ?? "general",
      description: d?.description ?? "",
      kind: d?.kind ?? "flat",
      provider: d?.provider ?? "",
      status:
        d?.status === "inactive" ? ("inactive" as const) : ("active" as const),
      visibility: d?.visibility ?? "shared",
      consumerPermission: d?.consumerPermission ?? "",
      items: original?.items ?? [],
      reason: "",
    },
  });
  const fields = useFieldArray({
    control: form.control,
    name: "items",
    keyName: "fieldKey",
  });
  const kind = useWatch({ control: form.control, name: "kind" });
  const watchedItems = useWatch({ control: form.control, name: "items" });
  return (
    <form
      className="admin-form gov-dictionary-form"
      onSubmit={form.handleSubmit((data) =>
        task.mutate(async () => {
          const definition: DictionaryDefinition = {
            id: d?.id ?? 0,
            target,
            revision: d?.revision ?? 0,
            code: data.code,
            nameEn: data.nameEn,
            nameZh: data.nameZh,
            category: data.category,
            description: data.description,
            kind: data.kind,
            provider: data.kind === "dynamic" ? data.provider : "",
            status: data.status,
            visibility: data.visibility,
            consumerPermission: data.consumerPermission,
            allowExtensions: false,
          };
          const response = await dictionariesDraft({
            document: {
              definition,
              items: data.kind === "dynamic" ? [] : data.items,
            },
            baseRevision: d?.revision ?? 0,
            reason: data.reason,
          });
          if (response.status === 201)
            onSaved?.(response.data.document?.definition.id ?? 0);
          form.setValue("reason", "");
        }),
      )}
    >
      <section className="admin-panel entity-card gov-form-section">
        <div className="form-grid">
          {(["code", "nameEn", "nameZh", "category"] as const).map((key) => (
            <label key={key}>
              <span>{t(key)}</span>
              <Input
                {...form.register(key)}
                disabled={!writable || (key === "code" && !!d?.id)}
              />
            </label>
          ))}
          <label>
            <span>{t("kind")}</span>
            <select
              {...form.register("kind")}
              disabled={!writable || !!d?.revision}
            >
              {["flat", "tree", "dynamic"].map((v) => (
                <option value={v} key={v}>
                  {t(v)}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span>{t("status")}</span>
            <select {...form.register("status")} disabled={!writable}>
              <option value="active">{t("active")}</option>
              <option value="inactive">{t("inactive")}</option>
              <option value="archived">{t("archived")}</option>
            </select>
          </label>
          {kind === "dynamic" && (
            <label>
              <span>{t("provider")}</span>
              <select
                {...form.register("provider")}
                disabled={!writable || !!d?.revision}
              >
                <option value="">{t("select")}</option>
                {["organizations", "departments", "roles", "accounts"].map(
                  (p) => (
                    <option key={p} value={p}>
                      {t(`admin:nav.${p}`)}
                    </option>
                  ),
                )}
              </select>
            </label>
          )}
          <label>
            <span>{t("visibility")}</span>
            <select {...form.register("visibility")} disabled={!writable}>
              <option value="shared">{t("shared")}</option>
              <option value="management">{t("management")}</option>
            </select>
          </label>
          <label>
            <span>{t("consumerPermission")}</span>
            <select
              {...form.register("consumerPermission")}
              disabled={!writable}
            >
              <option value="">{t("default")}</option>
              {admin.catalog.map((p) => (
                <option key={p.code} value={p.code}>
                  {p.code}
                </option>
              ))}
            </select>
          </label>
        </div>
        <label>
          <span>{t("description")}</span>
          <Input {...form.register("description")} disabled={!writable} />
        </label>
      </section>
      {kind !== "dynamic" && (
        <section className="admin-panel entity-card">
          <header className="gov-card-heading gov-line">
            <h2>{t("items")}</h2>
            {writable && !d?.allowExtensions && (
              <Button
                type="button"
                variant="outline"
                onClick={() => fields.append({ ...emptyItem })}
              >
                <Plus size={15} />
                {t("addItem")}
              </Button>
            )}
          </header>
          <div className="gov-dictionary-items">
            {fields.fields.length === 0 && (
              <p className="gov-empty">{t("empty")}</p>
            )}
            {fields.fields.map((item, index) => {
              const stable = !!item.id;
              return (
                <details
                  key={item.fieldKey}
                  open={!stable}
                  className="gov-item"
                >
                  <summary>
                    <code>{watchedItems[index]?.code || t("code")}</code>
                    <span>{watchedItems[index]?.labelEn}</span>
                  </summary>
                  <div className="form-grid">
                    {(
                      [
                        "code",
                        "labelEn",
                        "labelZh",
                        "parentCode",
                        "sort",
                        "startsAt",
                        "endsAt",
                        "color",
                        "icon",
                        "descriptionEn",
                        "descriptionZh",
                      ] as const
                    )
                      .filter((key) => key !== "parentCode" || kind === "tree")
                      .map((key) => (
                        <label key={key}>
                          <span>
                            {t(
                              key === "descriptionEn" || key === "descriptionZh"
                                ? key === "descriptionEn"
                                  ? "nameEn"
                                  : "nameZh"
                                : key,
                            )}
                          </span>
                          <Input
                            {...form.register(
                              `items.${index}.${key}`,
                              key === "sort" ? { valueAsNumber: true } : {},
                            )}
                            type={key === "sort" ? "number" : "text"}
                            disabled={!writable || (key === "code" && stable)}
                          />
                        </label>
                      ))}
                    <label>
                      <span>{t("status")}</span>
                      <select
                        {...form.register(`items.${index}.status`)}
                        disabled={!writable}
                      >
                        {["active", "inactive", "archived"].map((v) => (
                          <option key={v} value={v}>
                            {t(v)}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="checkbox-label">
                      <input
                        type="checkbox"
                        {...form.register(`items.${index}.isDefault`)}
                        disabled={!writable}
                      />
                      {t("isDefault")}
                    </label>
                  </div>
                  {!stable && writable && (
                    <Button
                      type="button"
                      variant="ghost"
                      onClick={() => fields.remove(index)}
                    >
                      {t("remove")}
                    </Button>
                  )}
                </details>
              );
            })}
          </div>
        </section>
      )}
      {writable && (
        <div className="gov-save">
          <label>
            <span>{t("reason")}</span>
            <Input {...form.register("reason")} placeholder={t("reasonHint")} />
          </label>
          <Button type="submit" disabled={task.isPending}>
            {task.isPending ? t("busy") : t("draft")}
          </Button>
        </div>
      )}
      {Object.keys(form.formState.errors).length > 0 && (
        <p role="alert" className="field-error">
          {t("invalid")}
        </p>
      )}
      {task.error && <ErrorBanner error={task.error} />}{" "}
      {task.isSuccess && <p role="status">{t("saved")}</p>}
    </form>
  );
}
export function DictionaryOptions({
  code,
  onSelect,
}: {
  code: string;
  onSelect?: (id: number) => void;
}) {
  const { t, i18n } = useTranslation("governance");
  const session = useSession();
  const q = useQuery({
    queryKey: useGovKey("options", code),
    queryFn: () =>
      dictionariesOptions(code, {
        language: i18n.resolvedLanguage === "zh-CN" ? "zh-CN" : "en",
      }),
    enabled: session.permissions.includes("dictionaries:use"),
    retry: false,
  });
  return (
    <section className="admin-panel entity-card gov-card-heading">
      <h2>{t("options")}</h2>
      {q.error ? (
        <ErrorBanner error={q.error} />
      ) : (
        <select
          aria-label={t("options")}
          defaultValue=""
          onChange={(e) => onSelect?.(Number(e.target.value))}
        >
          <option value="">{t("select")}</option>
          {q.data?.status === 200 &&
            q.data.data.map((o) => (
              <option
                key={`${o.source}:${o.id}`}
                value={o.id}
                disabled={o.disabled}
              >
                {o.parentCode ? `${o.parentCode} / ` : ""}
                {o.label}
              </option>
            ))}
        </select>
      )}
    </section>
  );
}
function ImportDialog({
  document,
  onClose,
}: {
  document: DictionaryDocument;
  onClose: () => void;
}) {
  const { t } = useTranslation("governance");
  const task = useGovTask();
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [content, setContent] = useState("");
  const form = useForm({
    resolver: zodResolver(z.object({ reason: z.string().min(5) })),
    defaultValues: { reason: "" },
  });
  const format = file?.name.toLowerCase().endsWith(".csv") ? "csv" : "json";
  return (
    <EditorDialog
      open
      wide
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t("import")}
      description={t("importHint")}
    >
      <form
        className="admin-form"
        onSubmit={form.handleSubmit((data) => {
          if (!preview || preview.errors.length) return;
          task.mutate(async () => {
            await dictionariesImportDraft(document.definition.id, {
              format,
              content,
              baseRevision: document.definition.revision,
              reason: data.reason,
            });
            onClose();
          });
        })}
      >
        <label>
          <span>{t("filename")}</span>
          <Input
            type="file"
            accept=".csv,.json"
            onChange={(e) => {
              setFile(e.target.files?.[0] ?? null);
              setPreview(null);
            }}
          />
        </label>
        <Button
          type="button"
          variant="outline"
          disabled={!file || task.isPending}
          onClick={() =>
            task.mutate(async () => {
              if (!file || file.size > 10 * 1024 * 1024) {
                form.setError("root", { message: "fileLimit" });
                return;
              }
              const text = await file.text();
              setContent(text);
              const r = await dictionariesImport(document.definition.id, {
                format,
                content: text,
              });
              if (r.status === 200) setPreview(r.data);
            })
          }
        >
          {t("preview")}
        </Button>
        {preview && (
          <>
            <p role="status">
              {t("addedUpdated", {
                added: preview.added,
                updated: preview.updated,
              })}
            </p>
            {preview.errors.length > 0 ? (
              <ul className="field-error" role="alert">
                {preview.errors.map((message, i) => {
                  const [row, field, reason] = message.split(":");
                  return (
                    <li key={i}>
                      {t("importError", {
                        row,
                        field,
                        reason: t(`importErrors.${reason}`, {
                          defaultValue: t("invalid"),
                        }),
                      })}
                    </li>
                  );
                })}
              </ul>
            ) : (
              <div className="gov-import-preview">
                {preview.document.items.slice(0, 100).map((item) => (
                  <p key={item.code}>
                    <code>{item.code}</code>
                    <span>{item.labelEn}</span>
                    <span>{item.labelZh}</span>
                  </p>
                ))}
              </div>
            )}
          </>
        )}
        <label>
          <span>{t("reason")}</span>
          <Input {...form.register("reason")} />
        </label>
        {Object.keys(form.formState.errors).length > 0 && (
          <p role="alert">
            {t(form.formState.errors.root?.message ?? "invalid")}
          </p>
        )}
        {task.error && <ErrorBanner error={task.error} />}
        <div className="form-actions">
          <Button type="button" variant="outline" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button
            type="submit"
            disabled={!preview || preview.errors.length > 0 || task.isPending}
          >
            {t("importDraft")}
          </Button>
        </div>
      </form>
    </EditorDialog>
  );
}
