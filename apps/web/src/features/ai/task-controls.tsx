import { useTranslation } from "react-i18next";
import { ListFilter } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
export type TaskSort = "recent" | "oldest" | "title";
export function TaskControls({
  view,
  setView,
  sort,
  setSort,
  activeCount,
  archivedCount,
}: {
  view: "active" | "archived";
  setView: (view: "active" | "archived") => void;
  sort: TaskSort;
  setSort: (sort: TaskSort) => void;
  activeCount: number;
  archivedCount: number;
}) {
  const { t } = useTranslation("ai");
  return (
    <div className="ai-tasks-tools">
      <div className="ai-task-views">
        <Button
          variant="ghost"
          size="sm"
          type="button"
          aria-pressed={view === "active"}
          onClick={() => setView("active")}
        >
          {t("tasks.views.active")}
          <small>{activeCount}</small>
        </Button>
        <Button
          variant="ghost"
          size="sm"
          type="button"
          aria-pressed={view === "archived"}
          onClick={() => setView("archived")}
        >
          {t("tasks.views.archived")}
          <small>{archivedCount}</small>
        </Button>
      </div>
      <div className="flex items-center gap-2 text-muted-foreground">
        <ListFilter className="size-4 shrink-0" aria-hidden="true" />
        <Select
          value={sort}
          onValueChange={(value) => {
            if (value === "recent" || value === "oldest" || value === "title")
              setSort(value);
          }}
        >
          <SelectTrigger aria-label={t("tasks.sort.label")} className="w-full">
            <SelectValue>{t(`tasks.sort.${sort}`)}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            {(["recent", "oldest", "title"] as const).map((value) => (
              <SelectItem key={value} value={value}>
                {t(`tasks.sort.${value}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}
