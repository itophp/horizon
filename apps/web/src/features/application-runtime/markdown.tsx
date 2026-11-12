import { useTranslation } from "react-i18next";
import { lazy, Suspense } from "react";
import Markdown, { defaultUrlTransform } from "react-markdown";
import remarkGfm from "remark-gfm";
import { Skeleton } from "@/components/ui/skeleton";
import type { ControlProps } from "./components";
const Editor = lazy(() =>
  import("@/features/governance/notification-editor").then((m) => ({
    default: m.NotificationEditor,
  })),
);
export function MarkdownControl(props: ControlProps) {
  const { t } = useTranslation();
  return (
    <div aria-label={t(props.definition.labelKey)}>
      <Suspense fallback={<Skeleton className="h-40 w-full" />}>
        <Editor
          inputId={props.id}
          ariaLabel={t(props.definition.labelKey)}
          onBlur={props.onBlur}
          value={String(props.value ?? "")}
          onChange={(value) => props.onChange(value)}
          readOnly={props.disabled}
        />
      </Suspense>
    </div>
  );
}
export function MarkdownDisplay({ value }: { value: unknown }) {
  return (
    <div className="runtime-markdown">
      <Markdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        urlTransform={(url) =>
          /^(https?:\/\/|mailto:|\/[^/]|#)/i.test(url)
            ? defaultUrlTransform(url)
            : ""
        }
        components={{
          a: ({ href, children }) => (
            <a href={href} rel="noopener noreferrer" target="_blank">
              {children}
            </a>
          ),
          img: ({ alt }) => <span>{alt}</span>,
        }}
      >
        {String(value ?? "")}
      </Markdown>
    </div>
  );
}
