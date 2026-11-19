import { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/api/client/request";
import {
  ErrorBanner,
  Loading,
  ShortDate,
} from "@/features/administration/shared";
import {
  mailAttachmentRead,
  mailConnectionTest,
  mailCredentialReplace,
  mailDeliveryList,
  mailDeliveryCancel,
  mailDeliveryRetry,
  mailMailboxDisable,
  mailMailboxEnable,
  mailMailboxList,
  mailMailboxOnboard,
  mailMailboxUpdate,
  mailMessageGet,
  mailMessageList,
  mailMessageMarkRead,
  mailMessageSend,
  type MailAddress,
  type Mailbox,
  filesUpload,
} from "@/lib/capabilities";
import { EmailHtml } from "./email-html";
import type { ComponentRendererProps } from "./component-registry";

const onboardSchema = z.object({
  name: z.string().min(1),
  emailAddress: z.string().email(),
  password: z.string().min(1),
  smtpHost: z.string().min(1),
  smtpPort: z.number().int().min(1).max(65535),
  smtpUsername: z.string(),
  pop3Host: z.string().min(1),
  pop3Port: z.number().int().min(1).max(65535),
  pop3Username: z.string(),
});
type OnboardForm = z.infer<typeof onboardSchema>;

const mailboxUpdateSchema = onboardSchema.omit({ password: true }).extend({
  id: z.number().int().positive(),
  revision: z.number().int().positive(),
  defaultSend: z.boolean(),
});
type MailboxUpdateForm = z.infer<typeof mailboxUpdateSchema>;

const composeSchema = z.object({
  mailboxId: z.number().int().positive(),
  to: z.string().min(3),
  cc: z.string(),
  subject: z.string().min(1),
  markdown: z.string().min(1),
});
type ComposeForm = z.infer<typeof composeSchema>;
type ComposeAttachment = { fileId: number; filename: string };

function hostFor(email: string, prefix: string) {
  const domain = email.split("@")[1]?.toLowerCase().trim();
  return domain ? `${prefix}.${domain}` : "";
}

function parseAddresses(value: string): MailAddress[] {
  return value
    .split(/[,;]/)
    .map((item) => item.trim())
    .filter(Boolean)
    .map((address) => ({ address }));
}

function readBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string")
        resolve(reader.result.split(",")[1] ?? "");
      else reject(new Error("file_unavailable"));
    };
    reader.onerror = () => reject(new Error("file_unavailable"));
    reader.readAsDataURL(file);
  });
}

function AutoHostForm({ onSaved }: { onSaved: () => void }) {
  const { t } = useTranslation("mail");
  const [notice, setNotice] = useState("");
  const form = useForm<OnboardForm>({
    resolver: zodResolver(onboardSchema),
    defaultValues: {
      name: "",
      emailAddress: "",
      password: "",
      smtpHost: "",
      smtpPort: 587,
      smtpUsername: "",
      pop3Host: "",
      pop3Port: 995,
      pop3Username: "",
    },
  });
  const email = useWatch({ control: form.control, name: "emailAddress" });
  useEffect(() => {
    const smtpHost = hostFor(email, "smtp");
    const pop3Host = hostFor(email, "pop");
    if (smtpHost && !form.getFieldState("smtpHost").isTouched)
      form.setValue("smtpHost", smtpHost);
    if (pop3Host && !form.getFieldState("pop3Host").isTouched)
      form.setValue("pop3Host", pop3Host);
    if (email && !form.getFieldState("smtpUsername").isTouched)
      form.setValue("smtpUsername", email);
    if (email && !form.getFieldState("pop3Username").isTouched)
      form.setValue("pop3Username", email);
  }, [email, form]);
  const submit = form.handleSubmit(async (values) => {
    const result = await mailMailboxOnboard({ ...values, smtpTls: "starttls" });
    if (result.data.status === "unverified") {
      setNotice(t("mailbox.verifyFailed"));
      onSaved();
      return;
    }
    setNotice("");
    form.reset();
    onSaved();
  });
  return (
    <form className="mail-form" onSubmit={submit}>
      <label htmlFor="mail-name">{t("mailbox.name")}</label>
      <input id="mail-name" {...form.register("name")} />
      <label htmlFor="mail-address">{t("mailbox.address")}</label>
      <input
        id="mail-address"
        type="email"
        {...form.register("emailAddress")}
      />
      <label htmlFor="mail-password">{t("mailbox.password")}</label>
      <input
        id="mail-password"
        type="password"
        autoComplete="new-password"
        {...form.register("password")}
      />
      <label htmlFor="mail-smtp-host">{t("mailbox.smtpHost")}</label>
      <input id="mail-smtp-host" {...form.register("smtpHost")} />
      <label htmlFor="mail-pop-host">{t("mailbox.popHost")}</label>
      <input id="mail-pop-host" {...form.register("pop3Host")} />
      <Button type="submit">{t("mailbox.connect")}</Button>
      <p className="mail-hint">{t("mailbox.autoHint")}</p>
      {notice ? <p role="status">{notice}</p> : null}
    </form>
  );
}

function MailboxEditor({
  mailbox,
  onSaved,
}: {
  mailbox: Mailbox;
  onSaved: () => void;
}) {
  const { t } = useTranslation("mail");
  const send = mailbox.channels.find((channel) => channel.direction === "send");
  const receive = mailbox.channels.find(
    (channel) => channel.direction === "receive",
  );
  const form = useForm<MailboxUpdateForm>({
    resolver: zodResolver(mailboxUpdateSchema),
    defaultValues: {
      id: mailbox.id,
      revision: mailbox.revision,
      name: mailbox.name,
      emailAddress: mailbox.emailAddress,
      defaultSend: mailbox.defaultSend,
      smtpHost: send?.host ?? "",
      smtpPort: send?.port ?? 587,
      smtpUsername: send?.username ?? mailbox.emailAddress,
      pop3Host: receive?.host ?? "",
      pop3Port: receive?.port ?? 995,
      pop3Username: receive?.username ?? mailbox.emailAddress,
    },
  });
  const email = useWatch({ control: form.control, name: "emailAddress" });
  const [smtpPassword, setSmtpPassword] = useState("");
  const [popPassword, setPopPassword] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const smtpHost = hostFor(email, "smtp");
    const pop3Host = hostFor(email, "pop");
    if (smtpHost && !form.getValues("smtpHost"))
      form.setValue("smtpHost", smtpHost);
    if (pop3Host && !form.getValues("pop3Host"))
      form.setValue("pop3Host", pop3Host);
  }, [email, form]);
  const submit = form.handleSubmit(async (values) => {
    try {
      await mailMailboxUpdate({ ...values, smtpTls: "starttls" });
      if (smtpPassword)
        await mailCredentialReplace(mailbox.id, "smtp", smtpPassword);
      if (popPassword)
        await mailCredentialReplace(mailbox.id, "pop3", popPassword);
      setNotice("");
      onSaved();
    } catch (error) {
      if (error instanceof ApiError && error.code === "approval_required") {
        setNotice(t("mailbox.credentialApprovalRequired"));
        return;
      }
      throw error;
    }
  });
  return (
    <form className="mail-form" onSubmit={submit}>
      <label htmlFor={`mail-name-${mailbox.id}`}>{t("mailbox.name")}</label>
      <input id={`mail-name-${mailbox.id}`} {...form.register("name")} />
      <label htmlFor={`mail-address-${mailbox.id}`}>
        {t("mailbox.address")}
      </label>
      <input
        id={`mail-address-${mailbox.id}`}
        type="email"
        {...form.register("emailAddress")}
      />
      <label htmlFor={`mail-smtp-${mailbox.id}`}>{t("mailbox.smtpHost")}</label>
      <input id={`mail-smtp-${mailbox.id}`} {...form.register("smtpHost")} />
      <label htmlFor={`mail-pop-${mailbox.id}`}>{t("mailbox.popHost")}</label>
      <input id={`mail-pop-${mailbox.id}`} {...form.register("pop3Host")} />
      <label htmlFor={`mail-smtp-password-${mailbox.id}`}>
        {t("mailbox.newSmtpPassword")}
      </label>
      <input
        id={`mail-smtp-password-${mailbox.id}`}
        type="password"
        autoComplete="new-password"
        value={smtpPassword}
        onChange={(event) => setSmtpPassword(event.target.value)}
      />
      <label htmlFor={`mail-pop-password-${mailbox.id}`}>
        {t("mailbox.newPopPassword")}
      </label>
      <input
        id={`mail-pop-password-${mailbox.id}`}
        type="password"
        autoComplete="new-password"
        value={popPassword}
        onChange={(event) => setPopPassword(event.target.value)}
      />
      <Button type="submit">{t("mailbox.save")}</Button>
      {notice ? <p role="status">{notice}</p> : null}
    </form>
  );
}

function MailboxesPanel() {
  const { t } = useTranslation("mail");
  const [editing, setEditing] = useState<Mailbox | null>(null);
  const mailboxes = useQuery({
    queryKey: ["mail", "mailboxes"],
    queryFn: ({ signal }) => mailMailboxList(signal),
  });
  if (mailboxes.isPending) return <Loading />;
  if (mailboxes.isError) return <ErrorBanner error={mailboxes.error} />;
  return (
    <section className="mail-panel">
      <AutoHostForm onSaved={() => void mailboxes.refetch()} />
      {editing ? (
        <MailboxEditor
          mailbox={editing}
          onSaved={() => {
            setEditing(null);
            void mailboxes.refetch();
          }}
        />
      ) : null}
      <ul className="mail-mailbox-list">
        {(mailboxes.data?.data.items ?? []).map((mailbox: Mailbox) => (
          <li key={mailbox.id}>
            <strong>{mailbox.name}</strong>
            <span>{mailbox.emailAddress}</span>
            <span>{t(`mailbox.status.${mailbox.status}`)}</span>
            {mailbox.defaultSend ? <span>{t("mailbox.default")}</span> : null}
            <span className="mail-actions">
              <Button type="button" onClick={() => setEditing(mailbox)}>
                {t("mailbox.edit")}
              </Button>
              {!mailbox.defaultSend ? (
                <Button
                  type="button"
                  onClick={() =>
                    void mailMailboxUpdate({
                      id: mailbox.id,
                      revision: mailbox.revision,
                      name: mailbox.name,
                      emailAddress: mailbox.emailAddress,
                      defaultSend: true,
                      smtpTls: "starttls",
                      smtpHost:
                        mailbox.channels.find(
                          (channel) => channel.direction === "send",
                        )?.host ?? "",
                      smtpPort:
                        mailbox.channels.find(
                          (channel) => channel.direction === "send",
                        )?.port ?? 587,
                      smtpUsername:
                        mailbox.channels.find(
                          (channel) => channel.direction === "send",
                        )?.username ?? mailbox.emailAddress,
                      pop3Host:
                        mailbox.channels.find(
                          (channel) => channel.direction === "receive",
                        )?.host ?? "",
                      pop3Port:
                        mailbox.channels.find(
                          (channel) => channel.direction === "receive",
                        )?.port ?? 995,
                      pop3Username:
                        mailbox.channels.find(
                          (channel) => channel.direction === "receive",
                        )?.username ?? mailbox.emailAddress,
                    }).then(() => mailboxes.refetch())
                  }
                >
                  {t("mailbox.makeDefault")}
                </Button>
              ) : null}
              <Button
                type="button"
                onClick={() =>
                  void testMailbox(mailbox.id).then(() => mailboxes.refetch())
                }
              >
                {t("mailbox.test")}
              </Button>
              {mailbox.status === "active" ? (
                <Button
                  type="button"
                  onClick={() =>
                    void mailMailboxDisable(mailbox.id).then(() =>
                      mailboxes.refetch(),
                    )
                  }
                >
                  {t("mailbox.disable")}
                </Button>
              ) : null}
              {mailbox.status === "disabled" ||
              mailbox.status === "unverified" ? (
                <Button
                  type="button"
                  onClick={() =>
                    void mailMailboxEnable(mailbox.id).then(() =>
                      mailboxes.refetch(),
                    )
                  }
                >
                  {t("mailbox.enable")}
                </Button>
              ) : null}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}

async function testMailbox(mailboxId: number) {
  await mailConnectionTest(mailboxId, "send");
  await mailConnectionTest(mailboxId, "receive");
}

function MessageRows({ folder }: { folder: "inbox" | "sent" }) {
  const { t } = useTranslation("mail");
  const navigate = useNavigate();
  const messages = useQuery({
    queryKey: ["mail", "messages", folder],
    queryFn: ({ signal }) =>
      mailMessageList({ folder, page: 1, pageSize: 50 }, signal),
  });
  if (messages.isPending) return <Loading />;
  if (messages.isError) return <ErrorBanner error={messages.error} />;
  return (
    <ul className="mail-list">
      {(messages.data?.data.items ?? []).map((message) => (
        <li key={message.id} className={message.read ? "read" : "unread"}>
          <button
            type="button"
            onClick={() => navigate(`/mail/${message.id}`)}
            className="mail-message-button"
          >
            <strong>{message.subject || t("message.noSubject")}</strong>
            <span>{message.from}</span>
            <ShortDate value={message.occurredAt} />
          </button>
        </li>
      ))}
    </ul>
  );
}

function ComposePanel() {
  const { t } = useTranslation("mail");
  const navigate = useNavigate();
  const location = useLocation();
  const preset = (location.state ?? {}) as {
    to?: string;
    cc?: string;
    subject?: string;
    markdown?: string;
    attachments?: ComposeAttachment[];
  };
  const mailboxes = useQuery({
    queryKey: ["mail", "mailboxes"],
    queryFn: ({ signal }) => mailMailboxList(signal),
  });
  const defaultMailbox = useMemo(() => {
    const active = (mailboxes.data?.data.items ?? []).filter(
      (item) => item.status === "active",
    );
    return active.find((item) => item.defaultSend) ?? active[0];
  }, [mailboxes.data]);
  const form = useForm<ComposeForm>({
    resolver: zodResolver(composeSchema),
    defaultValues: {
      mailboxId: defaultMailbox?.id ?? 0,
      to: preset.to ?? "",
      cc: preset.cc ?? "",
      subject: preset.subject ?? "",
      markdown: preset.markdown ?? "",
    },
  });
  const [notice, setNotice] = useState("");
  const [uploadError, setUploadError] = useState<unknown>();
  const [attachments, setAttachments] = useState<ComposeAttachment[]>(
    preset.attachments ?? [],
  );
  const [uploading, setUploading] = useState(false);
  useEffect(() => {
    if (defaultMailbox) form.setValue("mailboxId", defaultMailbox.id);
  }, [defaultMailbox, form]);
  const submit = form.handleSubmit(async (values) => {
    try {
      await mailMessageSend({
        clientKey: crypto.randomUUID(),
        mailboxId: values.mailboxId,
        to: parseAddresses(values.to),
        cc: parseAddresses(values.cc),
        subject: values.subject,
        markdown: values.markdown,
        fileIds: attachments.map((attachment) => attachment.fileId),
      });
      navigate("/mail/sent");
    } catch (error) {
      if (error instanceof ApiError && error.code === "approval_required") {
        setNotice(t("compose.approvalRequired"));
        return;
      }
      throw error;
    }
  });
  return (
    <form className="mail-form" onSubmit={submit}>
      <p className="mail-from">
        <span>{t("compose.from")}</span>
        <strong>
          {defaultMailbox?.emailAddress ?? t("compose.noMailbox")}
        </strong>
      </p>
      <label htmlFor="mail-to">{t("compose.to")}</label>
      <input id="mail-to" {...form.register("to")} />
      <label htmlFor="mail-cc">{t("compose.cc")}</label>
      <input id="mail-cc" {...form.register("cc")} />
      <label htmlFor="mail-subject">{t("compose.subject")}</label>
      <input id="mail-subject" {...form.register("subject")} />
      <label htmlFor="mail-body">{t("compose.body")}</label>
      <textarea id="mail-body" rows={12} {...form.register("markdown")} />
      <label htmlFor="mail-attachments">{t("compose.attachments")}</label>
      <input
        id="mail-attachments"
        type="file"
        multiple
        disabled={uploading || attachments.length >= 20}
        onChange={async (event) => {
          const files = Array.from(event.target.files ?? []);
          event.target.value = "";
          if (!files.length) return;
          setUploading(true);
          setUploadError(undefined);
          try {
            const uploaded = await Promise.all(
              files.map(async (file) => ({
                file,
                response: await filesUpload({
                  name: file.name,
                  purpose: "mail_attachment",
                  content: await readBase64(file),
                }),
              })),
            );
            setAttachments((current) => [
              ...current,
              ...uploaded.map((item) => ({
                fileId: item.response.data.id,
                filename: item.response.data.name,
              })),
            ]);
          } catch (error) {
            setUploadError(error);
          } finally {
            setUploading(false);
          }
        }}
      />
      <ul className="mail-attachments">
        {attachments.map((attachment) => (
          <li key={attachment.fileId}>
            <span>{attachment.filename}</span>
            <Button
              type="button"
              onClick={() =>
                setAttachments((current) =>
                  current.filter((item) => item.fileId !== attachment.fileId),
                )
              }
            >
              {t("compose.removeAttachment")}
            </Button>
          </li>
        ))}
      </ul>
      {uploadError !== undefined ? <ErrorBanner error={uploadError} /> : null}
      <Button type="submit">{t("compose.send")}</Button>
      {notice ? <p role="status">{notice}</p> : null}
    </form>
  );
}

function MessageDetailPanel() {
  const { t } = useTranslation("mail");
  const params = useParams();
  const navigate = useNavigate();
  const id = Number(params.id);
  const message = useQuery({
    queryKey: ["mail", "message", id],
    queryFn: ({ signal }) => mailMessageGet(id, signal),
    enabled: id > 0,
  });
  const mailboxes = useQuery({
    queryKey: ["mail", "mailboxes"],
    queryFn: ({ signal }) => mailMailboxList(signal),
  });
  useEffect(() => {
    if (message.data?.data && !message.data.data.read)
      void mailMessageMarkRead(id).then(() => message.refetch());
  }, [id, message]);
  if (message.isPending) return <Loading />;
  if (message.isError) return <ErrorBanner error={message.error} />;
  const record = message.data.data;
  const ownAddress =
    mailboxes.data?.data.items.find((item) => item.id === record.mailboxId)
      ?.emailAddress ?? "";
  const originalRecipients = record.recipients
    .filter((item) => item.kind === "to" || item.kind === "cc")
    .map((item) => item.address);
  const otherRecipients = originalRecipients.filter(
    (address) => address !== ownAddress,
  );
  const replyState = {
    to: record.from,
    subject: record.subject.startsWith("Re:")
      ? record.subject
      : `Re: ${record.subject}`,
    markdown: `\n\n> ${record.bodyText}`,
  };
  const allState = {
    to:
      record.from !== ownAddress && record.from
        ? record.from
        : (otherRecipients.shift() ?? ""),
    cc: otherRecipients.join(", "),
    subject: record.subject.startsWith("Re:")
      ? record.subject
      : `Re: ${record.subject}`,
    markdown: `\n\n> ${record.bodyText}`,
  };
  const forwardState = {
    subject: record.subject.startsWith("Fwd:")
      ? record.subject
      : `Fwd: ${record.subject}`,
    markdown: `\n\n> ${record.bodyText}`,
    attachments: record.attachments.map((attachment) => ({
      fileId: attachment.fileId,
      filename: attachment.filename,
    })),
  };
  return (
    <article className="mail-detail">
      <header>
        <h2>{record.subject || t("message.noSubject")}</h2>
        <p>{record.from}</p>
        <ShortDate value={record.occurredAt} />
      </header>
      <div className="mail-actions">
        <Button
          type="button"
          onClick={() => navigate("/mail/compose", { state: replyState })}
        >
          {t("message.reply")}
        </Button>
        <Button
          type="button"
          onClick={() => navigate("/mail/compose", { state: allState })}
        >
          {t("message.replyAll")}
        </Button>
        <Button
          type="button"
          onClick={() => navigate("/mail/compose", { state: forwardState })}
        >
          {t("message.forward")}
        </Button>
      </div>
      <EmailHtml html={record.bodyHtml} text={record.bodyText} />
      <ul className="mail-attachments">
        {record.attachments.map((attachment) => (
          <li key={attachment.id}>
            <Link
              to="#"
              onClick={(event) => {
                event.preventDefault();
                void downloadAttachment(
                  record.id,
                  attachment.id,
                  attachment.filename,
                );
              }}
            >
              {attachment.filename}
            </Link>
          </li>
        ))}
      </ul>
    </article>
  );
}

async function downloadAttachment(
  messageId: number,
  attachmentId: number,
  filename: string,
) {
  const result = await mailAttachmentRead(messageId, attachmentId);
  const binary = atob(result.data.content);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1)
    bytes[index] = binary.charCodeAt(index);
  const url = URL.createObjectURL(new Blob([bytes]));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

function DeliveryPanel() {
  const { t } = useTranslation("mail");
  const deliveries = useQuery({
    queryKey: ["mail", "deliveries"],
    queryFn: ({ signal }) => mailDeliveryList(signal),
  });
  if (deliveries.isPending) return <Loading />;
  if (deliveries.isError) return <ErrorBanner error={deliveries.error} />;
  return (
    <table className="mail-table">
      <thead>
        <tr>
          <th>{t("deliveries.subject")}</th>
          <th>{t("deliveries.statusLabel")}</th>
          <th>{t("deliveries.attempts")}</th>
          <th>{t("deliveries.createdAt")}</th>
          <th>{t("deliveries.actions")}</th>
        </tr>
      </thead>
      <tbody>
        {(deliveries.data?.data.items ?? []).map((delivery) => (
          <tr key={delivery.id}>
            <td>{delivery.subject}</td>
            <td>{t(`deliveries.status.${delivery.status}`)}</td>
            <td>{delivery.attempts}</td>
            <td>
              <ShortDate value={delivery.createdAt} />
            </td>
            <td className="mail-actions">
              {delivery.status === "failed" ||
              delivery.status === "ambiguous" ? (
                <Button
                  type="button"
                  onClick={() =>
                    void mailDeliveryRetry(delivery.id).then(() =>
                      deliveries.refetch(),
                    )
                  }
                >
                  {t("deliveries.retry")}
                </Button>
              ) : null}
              {delivery.status === "queued" ? (
                <Button
                  type="button"
                  onClick={() =>
                    void mailDeliveryCancel(delivery.id).then(() =>
                      deliveries.refetch(),
                    )
                  }
                >
                  {t("deliveries.cancel")}
                </Button>
              ) : null}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function MailWorkspace({ props }: ComponentRendererProps) {
  const mode = String(props.mode ?? "inbox");
  if (mode === "mailboxes") return <MailboxesPanel />;
  if (mode === "sent") return <MessageRows folder="sent" />;
  if (mode === "compose") return <ComposePanel />;
  if (mode === "detail") return <MessageDetailPanel />;
  if (mode === "deliveries") return <DeliveryPanel />;
  return <MessageRows folder="inbox" />;
}
