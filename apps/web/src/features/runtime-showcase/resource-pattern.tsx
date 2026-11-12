import { useState } from "react";
import { useTranslation } from "react-i18next";
import { FileText, Image, FolderOpen } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
const initial = [
  { name: "Delivery acceptance.pdf", type: "documents", size: "2.4 MB" },
  { name: "Workspace layout.svg", type: "images", size: "186 KB" },
  { name: "Project cost analysis.xlsx", type: "documents", size: "820 KB" },
  { name: "Architecture overview.svg", type: "images", size: "245 KB" },
  { name: "Release checklist.md", type: "documents", size: "12 KB" },
  { name: "Brand guidelines.pdf", type: "documents", size: "4.2 MB" },
];
export function ResourcePattern() {
  const { t } = useTranslation("showcase");
  const [files, setFiles] = useState(initial);
  const [type, setType] = useState("allRecords");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<(typeof initial)[number] | null>(
    null,
  );
  return (
    <div className="sc-pattern-split">
      <aside className="sc-filter-nav">
        <h2>{t("folders")}</h2>
        {["allRecords", "documents", "images"].map((key) => (
          <Button
            variant={type === key ? "secondary" : "ghost"}
            key={key}
            onClick={() => setType(key)}
          >
            <FolderOpen size={15} />
            {t(key)}
          </Button>
        ))}
      </aside>
      <section className="sc-result-area">
        <div className="sc-toolbar">
          <Input
            aria-label={t("search")}
            placeholder={t("search")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <label className="sc-file-upload">
            {t("uploadLocal")}
            <Input
              aria-label={t("uploadLocal")}
              type="file"
              multiple
              onChange={(e) =>
                setFiles([
                  ...files,
                  ...Array.from(e.target.files ?? []).map((f) => ({
                    name: f.name,
                    type: f.type.startsWith("image/") ? "images" : "documents",
                    size: new Intl.NumberFormat().format(f.size) + " B",
                  })),
                ])
              }
            />
          </label>
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              {["fileName", "fileSize", "modified", "preview"].map((key) => (
                <TableHead key={key}>{t(key)}</TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {files
              .filter(
                (f) =>
                  (type === "allRecords" || type === f.type) &&
                  f.name.toLowerCase().includes(search.toLowerCase()),
              )
              .map((f) => (
                <TableRow key={f.name}>
                  <TableCell>
                    <span className="sc-file-name">
                      {f.type === "images" ? (
                        <Image size={18} />
                      ) : (
                        <FileText size={18} />
                      )}
                      <strong>{f.name}</strong>
                    </span>
                  </TableCell>
                  <TableCell>{f.size}</TableCell>
                  <TableCell>{t("sampleDate")}</TableCell>
                  <TableCell>
                    <Button variant="ghost" onClick={() => setSelected(f)}>
                      {t("preview")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
          </TableBody>
        </Table>
        <p className="sc-note">{t("uploadHint")}</p>
      </section>
      <Sheet
        open={!!selected}
        onOpenChange={(v) => {
          if (!v) setSelected(null);
        }}
      >
        <SheetContent showCloseButton={false} className="sc-resource-sheet">
          <SheetHeader>
            <SheetTitle>{selected?.name}</SheetTitle>
            <SheetDescription>{t("filePreview")}</SheetDescription>
          </SheetHeader>
          <Tabs defaultValue="preview">
            <TabsList>
              <TabsTrigger value="preview">{t("preview")}</TabsTrigger>
              <TabsTrigger value="detail">{t("detail")}</TabsTrigger>
            </TabsList>
            <TabsContent value="preview">
              <div className="sc-document-preview">
                <FileText size={40} />
                <h2>{selected?.name}</h2>
                <p>{t("detailsText")}</p>
                <hr />
                <h3>{t("milestones")}</h3>
                <p>{t("approvalImpact")}</p>
                <h3>{t("scope")}</h3>
                <p>{t("reviewDescription")}</p>
              </div>
            </TabsContent>
            <TabsContent value="detail">
              <dl className="sc-definition">
                <dt>{t("fileSize")}</dt>
                <dd>{selected?.size}</dd>
                <dt>{t("modified")}</dt>
                <dd>{t("sampleDate")}</dd>
                <dt>{t("owner")}</dt>
                <dd>{t("sampleOwner")}</dd>
              </dl>
            </TabsContent>
          </Tabs>
          <Button variant="outline" onClick={() => setSelected(null)}>
            {t("close")}
          </Button>
        </SheetContent>
      </Sheet>
    </div>
  );
}
