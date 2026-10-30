import { lazy, Suspense, useState } from "react";
import { X } from "lucide-react";
import { useController, useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  notificationsAudience,
  notificationsPreview,
  notificationsSend,
} from "@/api/generated/baseline";
import {
  ErrorBanner,
  Loading,
  EditorDialog,
} from "@/features/administration/shared";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useGovKey, useGovTask } from "./shared";
// 富文本编辑器只在发起通知页按需加载，避免把 Lexical/CodeMirror 带进主包。
const NotificationEditor = lazy(() =>
  import("./notification-editor").then((module) => ({
    default: module.NotificationEditor,
  })),
);
const schema = z.object({
  title: z.string().trim().min(1).max(200),
  markdown: z
    .string()
    .min(1)
    .max(50000)
    .refine((value) => value.trim().length > 0),
  targets: z.array(z.string()).min(1).max(200),
  email: z.boolean(),
});
type Values = z.infer<typeof schema>;
export function NotificationComposer({ onSent }: { onSent: () => void }) {
  const { t } = useTranslation("governance");
  const task = useGovTask();
  const [search, setSearch] = useState("");
  const [kind, setKind] = useState("account");
  const [clientKey, setClientKey] = useState(() => crypto.randomUUID());
  const [confirmation, setConfirmation] = useState<{
    values: Values;
    count: number;
  } | null>(null);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { title: "", markdown: "", targets: [], email: false },
  });
  // 编辑器只在挂载时读取初始 Markdown，之后的内容由它自己维护。
  const [initialMarkdown] = useState(() => form.getValues("markdown"));
  const { field: targetField } = useController({
    name: "targets",
    control: form.control,
  });
  const q = useQuery({
    queryKey: useGovKey("notification-audience"),
    queryFn: () => notificationsAudience(),
    retry: false,
  });
  const audience = q.data?.status === 200 ? q.data.data : [];
  const targets = (keys: string[]) =>
    keys.map((key) => {
      const found = audience.find((a) => `${a.kind}:${a.id}` === key);
      if (!found) throw new Error("recipient_unavailable");
      return { kind: found.kind, id: found.id };
    });
  return (
    <section className="admin-panel entity-card notification-composer">
      <header className="gov-card-heading">
        <h2>{t("messageContent")}</h2>
        <p>{t("messageContentHint")}</p>
      </header>
      <form
        className="notification-form"
        onSubmit={form.handleSubmit((data) =>
          task.mutate(async () => {
            const response = await notificationsPreview({
              targets: targets(data.targets),
            });
            if (response.status === 200)
              setConfirmation({
                values: data,
                count: response.data.recipientCount,
              });
          }),
        )}
      >
        <label>
          <span>{t("notificationTitle")}</span>
          <Input {...form.register("title")} maxLength={200} />
        </label>
        <fieldset className="notification-audience">
          <legend>{t("recipients")}</legend>
          <p className="muted-text">{t("audienceHint")}</p>
          <div className="gov-toolbar">
            <label>
              <span>{t("recipientKind")}</span>
              <select value={kind} onChange={(e) => setKind(e.target.value)}>
                {["account", "department", "company", "enterprise"].map((k) => (
                  <option key={k} value={k}>
                    {t(`recipientKinds.${k}`)}
                  </option>
                ))}
              </select>
            </label>
            <label>
              <span>{t("searchRecipients")}</span>
              <Input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </label>
          </div>
          <p>{t("selectedTargets", { count: targetField.value.length })}</p>
          {q.isPending ? (
            <Loading />
          ) : q.error ? (
            <ErrorBanner error={q.error} />
          ) : (
            <div className="notification-target-list">
              {audience
                .filter(
                  (a) =>
                    a.kind === kind &&
                    a.name
                      .toLocaleLowerCase()
                      .includes(search.toLocaleLowerCase()),
                )
                .map((a) => (
                  <label key={`${a.kind}:${a.id}`}>
                    <input
                      type="checkbox"
                      value={`${a.kind}:${a.id}`}
                      name={targetField.name}
                      checked={targetField.value.includes(`${a.kind}:${a.id}`)}
                      onBlur={targetField.onBlur}
                      onChange={(event) =>
                        targetField.onChange(
                          event.target.checked
                            ? [...targetField.value, `${a.kind}:${a.id}`]
                            : targetField.value.filter(
                                (key) => key !== `${a.kind}:${a.id}`,
                              ),
                        )
                      }
                    />
                    <span>{a.name}</span>
                    <small>
                      {t("recipientCount", { count: a.recipients })}
                    </small>
                  </label>
                ))}
            </div>
          )}
          {targetField.value.length > 0 && (
            <div className="notification-selected">
              {targetField.value.map((key) => (
                <Button
                  key={key}
                  type="button"
                  variant="outline"
                  onClick={() =>
                    form.setValue(
                      "targets",
                      targetField.value.filter((v) => v !== key),
                    )
                  }
                >
                  {audience.find((a) => `${a.kind}:${a.id}` === key)?.name}
                  <X size={12} aria-hidden="true" />
                </Button>
              ))}
            </div>
          )}
        </fieldset>
        <div className="notification-body">
          <span className="notification-body-label">{t("markdownBody")}</span>
          <Suspense fallback={<Loading />}>
            <NotificationEditor
              value={initialMarkdown}
              readOnly={task.isPending}
              onChange={(markdown) =>
                form.setValue("markdown", markdown, {
                  shouldDirty: true,
                  shouldValidate: form.formState.isSubmitted,
                })
              }
            />
          </Suspense>
          <p className="muted-text">{t("editorLimit")}</p>
        </div>
        <label className="notification-email">
          <input type="checkbox" {...form.register("email")} />
          <span>{t("alsoEmail")}</span>
        </label>
        {Object.keys(form.formState.errors).length > 0 && (
          <p role="alert">{t("notificationValidation")}</p>
        )}
        {task.error && <ErrorBanner error={task.error} />}
        <div className="form-actions">
          <Button type="submit" disabled={task.isPending}>
            {t("reviewSend")}
          </Button>
        </div>
      </form>
      {confirmation && (
        <EditorDialog
          title={t("confirmSend")}
          description={t("confirmSendHint", { count: confirmation.count })}
          open
          onOpenChange={(open) => {
            if (!open) setConfirmation(null);
          }}
        >
          <strong>{confirmation.values.title}</strong>
          <p>{t("sendSnapshotHint")}</p>
          {task.error && <ErrorBanner error={task.error} />}
          <div className="form-actions">
            <Button variant="outline" onClick={() => setConfirmation(null)}>
              {t("cancel")}
            </Button>
            <Button
              disabled={task.isPending}
              onClick={() =>
                task.mutate(async () => {
                  const response = await notificationsSend({
                    ...confirmation.values,
                    targets: targets(confirmation.values.targets),
                    clientKey,
                    fileIds: [],
                  });
                  if (response.status === 201) {
                    setConfirmation(null);
                    form.reset();
                    setClientKey(crypto.randomUUID());
                    onSent();
                  }
                })
              }
            >
              {t("sendNotification")}
            </Button>
          </div>
        </EditorDialog>
      )}
    </section>
  );
}
