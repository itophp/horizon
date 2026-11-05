import { FolderPlus, Upload } from "lucide-react";
import { useRef } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { ListSearch } from "@/features/administration/shared";
import { DriveSelect } from "./select";
export function DriveToolbar({
  search,
  kind,
  sort,
  filter,
  canEdit,
  uploading,
  resuming,
  onCreate,
  onUpload,
}: {
  search: string;
  kind: string;
  sort: string;
  filter: (key: string, value: string) => void;
  canEdit: boolean;
  uploading: boolean;
  resuming: boolean;
  onCreate: () => void;
  onUpload: (files: File[]) => void;
}) {
  const { t } = useTranslation(["drive", "admin"]);
  const fileInput = useRef<HTMLInputElement>(null);
  return (
    <div className="standalone-toolbar drive-toolbar">
      <div className="drive-toolbar-filters">
        <ListSearch value={search} onChange={(value) => filter("q", value)} />
        <DriveSelect
          label={t("drive:kind")}
          value={kind}
          onValueChange={(value) => filter("type", value)}
          options={[
            { value: "all", label: t("admin:actions.all") },
            { value: "folder", label: t("drive:folder") },
            { value: "file", label: t("drive:file") },
          ]}
        />
        <DriveSelect
          label={t("drive:sort")}
          value={sort}
          onValueChange={(value) => filter("sort", value)}
          options={[
            { value: "name", label: t("drive:sortName") },
            { value: "size", label: t("drive:sortSize") },
            { value: "uploaded", label: t("drive:sortUploaded") },
          ]}
        />
      </div>
      {canEdit && (
        <div className="toolbar-right">
          <Button type="button" variant="outline" onClick={onCreate}>
            <FolderPlus />
            {t("drive:createFolder")}
          </Button>
          <input
            ref={fileInput}
            type="file"
            multiple
            hidden
            aria-label={t("drive:upload")}
            onChange={(event) => {
              const selected = Array.from(event.target.files ?? []);
              event.target.value = "";
              if (selected.length) {
                onUpload(selected);
              }
            }}
          />
          <Button
            type="button"
            disabled={uploading}
            onClick={() => fileInput.current?.click()}
          >
            <Upload />
            {resuming ? t("drive:selectSameFile") : t("drive:upload")}
          </Button>
        </div>
      )}
    </div>
  );
}
