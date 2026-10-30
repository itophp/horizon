import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import {
  BlockTypeSelect,
  BoldItalicUnderlineToggles,
  CreateLink,
  InsertTable,
  InsertThematicBreak,
  ListsToggle,
  MDXEditor,
  Separator,
  StrikeThroughSupSubToggles,
  UndoRedo,
  headingsPlugin,
  linkDialogPlugin,
  linkPlugin,
  listsPlugin,
  markdownShortcutPlugin,
  quotePlugin,
  tablePlugin,
  thematicBreakPlugin,
  toolbarPlugin,
  type Translation,
} from "@mdxeditor/editor";
import "@mdxeditor/editor/style.css";

// 通知正文只允许常规外链协议，和 NotificationMarkdown 的 URL 处理保持一致。
const safeUrl = (url: string) => /^(https?:\/\/|mailto:)/i.test(url);

// 工具栏按需裁剪：不提供上传/插入文件（附件入口已移除），也不提供源码、
// 差异和预览浏览模式，编辑区本身就是所见即所得的结果。
const plugins = [
  headingsPlugin({ allowedHeadingLevels: [1, 2, 3] }),
  listsPlugin(),
  quotePlugin(),
  thematicBreakPlugin(),
  linkPlugin({ validateUrl: safeUrl }),
  linkDialogPlugin(),
  tablePlugin(),
  markdownShortcutPlugin(),
  toolbarPlugin({
    toolbarContents: () => (
      <>
        <UndoRedo />
        <Separator />
        <BlockTypeSelect />
        <BoldItalicUnderlineToggles options={["Bold", "Italic", "Underline"]} />
        <StrikeThroughSupSubToggles options={["Strikethrough"]} />
        <Separator />
        <ListsToggle options={["bullet", "number"]} />
        <Separator />
        <CreateLink />
        <InsertTable />
        <InsertThematicBreak />
      </>
    ),
  }),
];

export function NotificationEditor({
  value,
  onChange,
  readOnly,
}: {
  value: string;
  onChange: (markdown: string) => void;
  readOnly?: boolean;
}) {
  const { t } = useTranslation("governance");
  const translate = useCallback<Translation>(
    (key, fallback, interpolations) => {
      let text = t(`notificationEditor.${key}`, { defaultValue: fallback });
      for (const [name, item] of Object.entries(interpolations ?? {})) {
        text = text.replaceAll(`{{${name}}}`, String(item));
      }
      return text;
    },
    [t],
  );
  return (
    <div className="notification-editor-shell">
      <MDXEditor
        className="notification-editor notification-editor-surface"
        contentEditableClassName="notification-editor-content"
        markdown={value}
        onChange={(markdown) => onChange(markdown)}
        placeholder={
          <span className="notification-editor-placeholder">
            {t("markdownHint")}
          </span>
        }
        translation={translate}
        readOnly={readOnly}
        suppressHtmlProcessing
        plugins={plugins}
      />
    </div>
  );
}
