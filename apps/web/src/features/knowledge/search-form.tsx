import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export function KnowledgeSearchForm({
  keyword,
  onSearch,
}: {
  keyword: string;
  onSearch: (keyword: string) => void;
}) {
  const { t } = useTranslation("knowledge");
  const [draft, setDraft] = useState(keyword);
  return (
    <form
      role="search"
      aria-label={t("list.search")}
      className="mb-3 flex flex-wrap items-center gap-2"
      onSubmit={(event) => {
        event.preventDefault();
        onSearch(draft.trim());
      }}
    >
      <Input
        className="min-w-48 flex-1"
        type="search"
        aria-label={t("list.keyword")}
        placeholder={t("list.keyword")}
        maxLength={200}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
      />
      <Button type="submit">
        <Search aria-hidden="true" />
        {t("list.search")}
      </Button>
      {keyword && (
        <Button
          type="button"
          variant="ghost"
          onClick={() => {
            setDraft("");
            onSearch("");
          }}
        >
          <X aria-hidden="true" />
          {t("list.clear")}
        </Button>
      )}
    </form>
  );
}
