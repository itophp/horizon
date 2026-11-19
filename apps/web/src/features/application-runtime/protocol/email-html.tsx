import { useTranslation } from "react-i18next";

export function EmailHtml({ html, text }: { html?: string; text: string }) {
  const { t } = useTranslation("mail");
  if (html) {
    return (
      <iframe
        className="mail-html-frame"
        title={t("message.body")}
        sandbox="allow-popups allow-popups-to-escape-sandbox"
        srcDoc={html}
      />
    );
  }
  return <pre className="mail-plain-body">{text}</pre>;
}
