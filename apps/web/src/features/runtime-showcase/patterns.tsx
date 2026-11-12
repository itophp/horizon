import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Choice, Panel, Status } from "./primitives";
import { projects } from "./catalog";
import { Workbench } from "./views";
export function LayoutPatterns() {
  const { t } = useTranslation("showcase");
  const [selected, setSelected] = useState<number | null>(null);
  return (
    <Tabs defaultValue="workspace">
      <TabsList>
        <TabsTrigger value="workspace">{t("workspace")}</TabsTrigger>
        <TabsTrigger value="calendar">{t("calendar")}</TabsTrigger>
        <TabsTrigger value="gantt">{t("gantt")}</TabsTrigger>
      </TabsList>
      <TabsContent value="workspace">
        <Workbench engine={3} />
      </TabsContent>
      <TabsContent value="calendar">
        <h2 className="sc-section-title">{t("month")}</h2>
        <div className="sc-calendar">
          {Array.from({ length: 30 }, (_, i) => (
            <Button key={i} variant="outline" onClick={() => setSelected(i)}>
              <span>{i + 1}</span>
              {i % 4 === 0 && <small>{projects[i % 12]}</small>}
            </Button>
          ))}
        </div>
      </TabsContent>
      <TabsContent value="gantt">
        <Panel title={t("gantt")}>
          <div className="sc-gantt">
            {projects.slice(0, 6).map((n, i) => (
              <div key={n}>
                <Button variant="ghost" onClick={() => setSelected(i)}>
                  {n}
                </Button>
                <div>
                  <Button
                    aria-label={n}
                    onClick={() => setSelected(i)}
                    style={{ marginLeft: `${i * 9}%`, width: `${30 + i * 3}%` }}
                  >
                    {t(i % 2 ? "review" : "active")}
                  </Button>
                </div>
              </div>
            ))}
          </div>
          <p className="sc-muted">
            {t("date")}
            {t("patternLabel0")}
          </p>
        </Panel>
      </TabsContent>
      <Dialog
        open={selected !== null}
        onOpenChange={(v) => {
          if (!v) setSelected(null);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{projects[(selected ?? 0) % 12]}</DialogTitle>
            <DialogDescription>{t("detailsText")}</DialogDescription>
          </DialogHeader>
          <Status value="active" />
          <p>
            {t("milestones")}
            {t("patternLabel1")}
            {t("date")}
          </p>
        </DialogContent>
      </Dialog>
    </Tabs>
  );
}
export function ComponentPatterns() {
  const { t } = useTranslation("showcase");
  const [enabled, setEnabled] = useState(true);
  const [tags, setTags] = useState(["Design", "Operations"]);
  const [tag, setTag] = useState("");
  const [files, setFiles] = useState<string[]>([]);
  const [preview, setPreview] = useState(false);
  const [selected, setSelected] = useState("active");
  const [message, setMessage] = useState("");
  return (
    <>
      <div className="sc-grid-two">
        <Panel title={t("inputs")}>
          <label className="sc-field">
            {t("name")}
            <Input placeholder={t("name")} disabled={!enabled} />
          </label>
          <label className="sc-field">
            {t("notes")}
            <Textarea placeholder={t("notes")} disabled={!enabled} />
          </label>
          <label className="sc-field">
            {t("status")}
            <Choice
              labelKey="status"
              value={selected}
              options={["active", "review", "done"]}
              onChange={setSelected}
            />
          </label>
          <label className="sc-check">
            <Checkbox
              checked={enabled}
              onCheckedChange={(v) => setEnabled(v === true)}
            />
            {t("enabledControl")}
          </label>
          <div className="sc-toolbar">
            {tags.map((n) => (
              <Button
                variant="secondary"
                key={n}
                onClick={() => setTags(tags.filter((v) => v !== n))}
              >
                {n}
                {t("patternLabel2")}
              </Button>
            ))}
          </div>
          <div className="sc-toolbar">
            <Input
              value={tag}
              aria-label={t("tags")}
              placeholder={t("tags")}
              onChange={(e) => setTag(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && tag.trim()) {
                  setTags([...new Set([...tags, tag.trim()])]);
                  setTag("");
                }
              }}
            />
            <Button
              disabled={!tag.trim()}
              onClick={() => {
                setTags([...new Set([...tags, tag.trim()])]);
                setTag("");
              }}
            >
              {t("add")}
            </Button>
          </div>
        </Panel>
        <Panel title={t("interactions")}>
          <div className="sc-toolbar">
            <Button onClick={() => setPreview(true)}>{t("preview")}</Button>
            <DropdownMenu>
              <DropdownMenuTrigger render={<Button variant="outline" />}>
                {t("settings")}
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem onClick={() => setMessage(t("saved"))}>
                  {t("save")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setTags([])}>
                  {t("clear")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <Tooltip>
              <TooltipTrigger render={<Button variant="outline" />}>
                {t("help")}
              </TooltipTrigger>
              <TooltipContent>{t("tokenText")}</TooltipContent>
            </Tooltip>
          </div>
          <p role="status">{message}</p>
          <details open>
            <summary>{t("tree")}</summary>
            <ul className="sc-tree">
              <li>
                <details open>
                  <summary>{t("patternLabel3")}</summary>
                  <ul>
                    <li>
                      <Checkbox aria-label={t("patternLabel4")} />
                      {t("patternLabel5")}
                    </li>
                    <li>
                      <Checkbox aria-label={t("patternLabel6")} />
                      {t("patternLabel7")}
                    </li>
                  </ul>
                </details>
              </li>
              <li>
                <details>
                  <summary>{t("patternLabel8")}</summary>
                  <ul>
                    <li>{t("patternLabel9")}</li>
                    <li>{t("patternLabel10")}</li>
                  </ul>
                </details>
              </li>
            </ul>
          </details>
          <label className="sc-field">
            {t("upload")}
            <Input
              type="file"
              multiple
              onChange={(e) =>
                setFiles(Array.from(e.target.files ?? []).map((f) => f.name))
              }
            />
          </label>
          <ul>
            {files.map((f) => (
              <li key={f}>
                {f}
                <Button
                  variant="ghost"
                  onClick={() => setFiles(files.filter((n) => n !== f))}
                >
                  {t("remove")}
                </Button>
              </li>
            ))}
          </ul>
          <p className="sc-muted">{t("uploadHint")}</p>
        </Panel>
      </div>
      <Dialog open={preview} onOpenChange={setPreview}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("preview")}</DialogTitle>
            <DialogDescription>{t("tokenText")}</DialogDescription>
          </DialogHeader>
          <Status value={selected} />
          <p>{tags.join(" · ")}</p>
          <Button onClick={() => setPreview(false)}>{t("close")}</Button>
        </DialogContent>
      </Dialog>
    </>
  );
}
