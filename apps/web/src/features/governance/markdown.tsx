import Markdown, { defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import {
  filesContent,
  notificationsAttachment,
} from "@/api/generated/baseline";
import type { NotificationAttachment } from "@/api/generated/models";
import { Button } from "@/components/ui/button";
import { useGovKey, useGovTask, downloadContent } from "./shared";
import { ErrorBanner } from "@/features/administration/shared";
const fileID = (url: string) => /^file:([1-9]\d*)$/.exec(url)?.[1];
function content(id: number, messageId?: number) {
  return messageId ? notificationsAttachment(messageId, id) : filesContent(id);
}
function InlineImage({
  file,
  messageId,
}: {
  file: NotificationAttachment;
  messageId?: number;
}) {
  const q = useQuery({
    queryKey: useGovKey("message-image", messageId ?? 0, file.id),
    queryFn: () => content(file.id, messageId),
    retry: false,
  });
  return q.data?.status === 200 &&
    ["image/png", "image/jpeg", "image/webp"].includes(q.data.data.mime) ? (
    <img
      src={`data:${q.data.data.mime};base64,${q.data.data.content}`}
      alt={file.name}
    />
  ) : (
    <span>{file.name}</span>
  );
}
export function NotificationMarkdown({
  markdown,
  attachments,
  messageId,
}: {
  markdown: string;
  attachments: NotificationAttachment[];
  messageId?: number;
}) {
  const { t } = useTranslation("governance");
  const task = useGovTask();
  const download = (id: number) =>
    task.mutate(async () => {
      const r = await content(id, messageId);
      if (r.status === 200)
        downloadContent(
          r.data.name,
          Uint8Array.from(atob(r.data.content), (c) => c.charCodeAt(0)),
          r.data.mime,
        );
    });
  return (
    <div className="notification-markdown">
      <Markdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        urlTransform={(url) => (fileID(url) ? url : defaultUrlTransform(url))}
        components={{
          a: ({ href = "", children }) => {
            const id = Number(fileID(href));
            const file = attachments.find((f) => f.id === id);
            return file ? (
              <button
                type="button"
                className="notification-file-link"
                onClick={() => download(id)}
                disabled={task.isPending}
              >
                {children}
              </button>
            ) : id ? (
              <span>{children}</span>
            ) : (
              <a href={href} target="_blank" rel="noopener noreferrer">
                {children}
              </a>
            );
          },
          img: ({ src = "", alt = "" }) => {
            const file = attachments.find((f) => f.id === Number(fileID(src)));
            return file ? (
              <InlineImage file={file} messageId={messageId} />
            ) : (
              <span>{alt}</span>
            );
          },
        }}
      >
        {markdown}
      </Markdown>
      {attachments.length > 0 && (
        <div className="notification-files" aria-label={t("messageFiles")}>
          {attachments.map((file) => (
            <Button
              key={file.id}
              type="button"
              variant="outline"
              onClick={() => download(file.id)}
              disabled={task.isPending}
            >
              {file.name}
            </Button>
          ))}
        </div>
      )}
      {task.error && <ErrorBanner error={task.error} />}
    </div>
  );
}
