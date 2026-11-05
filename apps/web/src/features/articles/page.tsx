import { useParams } from "react-router";
import { ApplicationPage } from "@/features/application-runtime/page";
export function ArticlesPage() {
  const { id } = useParams();
  return (
    <ApplicationPage
      applicationId="articles"
      pageId={id ? "article.detail" : "article.list"}
    />
  );
}
