import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { tableFeatures, useTable } from "@tanstack/react-table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Choice, Status } from "./primitives";
import { projects } from "./catalog";
const features = tableFeatures({});
const noCreated: string[] = [];
const noRenamed: Record<string, string> = {};
export function Records({
  state,
  statusValue,
  onStatusChange,
  onReset,
  group = "allRecords",
  onNew,
  created = noCreated,
  renamed = noRenamed,
}: {
  state: string;
  statusValue?: string;
  onStatusChange?: (value: string) => void;
  onReset?: () => void;
  group?: string;
  onNew: () => void;
  created?: string[];
  renamed?: Record<string, string>;
}) {
  const { t } = useTranslation("showcase");
  const [q, setQ] = useSearchParams();
  const search = q.get("q") ?? "";
  const [searchInput, setSearchInput] = useState(search);
  const [previousSearch, setPreviousSearch] = useState(search);
  if (previousSearch !== search) {
    setPreviousSearch(search);
    setSearchInput(search);
  }
  const status = statusValue ?? q.get("status") ?? "all";
  const page = Number(q.get("p") ?? 1);
  const [selected, setSelected] = useState<number[]>([]);
  const [completed, setCompleted] = useState<number[]>([]);
  const [view, setView] = useState("list");
  const change = (key: string, value: string) => {
    const next = new URLSearchParams(q);
    next.set(key, value);
    next.set("p", "1");
    setQ(next);
  };
  const data = useMemo(
    () =>
      Array.from(
        { length: (state === "large" ? 1000 : 24) + created.length },
        (_, id) => ({
          id,
          name:
            renamed[id] ??
            created[id] ??
            projects[(id - created.length) % projects.length],
          status: completed.includes(id)
            ? "done"
            : ["active", "review", "done"][id % 3],
          amount: 24000 + id * 1350,
        }),
      )
        .filter(
          (x) =>
            (group === "allRecords" ||
              x.id % 3 ===
                [
                  "platformTeam",
                  "operationsTeam",
                  "deliveryTeam",
                  "internal",
                  "customer",
                  "partner",
                ].indexOf(group) %
                  3) &&
            x.name.toLowerCase().includes(search.toLowerCase()) &&
            (status === "all" || x.status === status),
        )
        .sort((a, b) => (q.get("sort") === "desc" ? b.id - a.id : a.id - b.id)),
    [state, completed, search, status, q, created, renamed, group],
  );
  const columns = useMemo(() => [{ id: "name", header: t("name") }], [t]);
  const table = useTable({
    features,
    data,
    columns,
    getRowId: (r) => String(r.id),
  });
  const pages = Math.max(1, Math.ceil(data.length / 8));
  const current = Math.max(1, Math.min(page, pages));
  const rows = table.getRowModel().rows.slice((current - 1) * 8, current * 8);
  return (
    <>
      <div className="sc-toolbar">
        <Input
          aria-label={t("search")}
          placeholder={t("search")}
          value={searchInput}
          onChange={(e) => {
            setSearchInput(e.target.value);
            change("q", e.target.value);
          }}
        />
        <Choice
          labelKey="status"
          value={status}
          options={["all", "active", "review", "done"]}
          onChange={(v) =>
            onStatusChange ? onStatusChange(v) : change("status", v)
          }
        />
        <Button
          variant="outline"
          onClick={() =>
            change("sort", q.get("sort") === "desc" ? "asc" : "desc")
          }
        >
          {t("sort")}
        </Button>
        <Tabs value={view} onValueChange={(v) => setView(String(v))}>
          <TabsList>
            <TabsTrigger value="list">{t("tabList")}</TabsTrigger>
            <TabsTrigger value="board">{t("tabBoard")}</TabsTrigger>
          </TabsList>
        </Tabs>
        <Button
          disabled={state === "readonly" || state === "disabled"}
          onClick={onNew}
        >
          {t("new")}
        </Button>
      </div>
      {selected.length > 0 && (
        <div className="sc-selection">
          {t("selected", { count: selected.length })}
          <Button
            variant="outline"
            disabled={state === "readonly"}
            onClick={() => {
              setCompleted([...completed, ...selected]);
              setSelected([]);
            }}
          >
            {t("batch")}
          </Button>
        </div>
      )}
      {view === "board" ? (
        <div className="sc-board">
          {["active", "review", "done"].map((s) => (
            <section key={s}>
              <h2>{t(s)}</h2>
              {data
                .filter((r) => r.status === s)
                .slice(0, 8)
                .map((r) => (
                  <Link
                    key={r.id}
                    to={`?pattern=recordDetail&record=${r.id}`}
                    className="sc-work-item"
                  >
                    <small>
                      {t("projectCodePrefix")}
                      {String(r.id + 1).padStart(3, "0")}
                    </small>
                    <strong>{r.name}</strong>
                    <Status value={s} />
                  </Link>
                ))}
            </section>
          ))}
        </div>
      ) : (
        <div className="sc-table-scroll">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>
                  <Checkbox
                    aria-label={t("selected", { count: data.length })}
                    checked={
                      data.length > 0 &&
                      data.every((r) => selected.includes(r.id))
                    }
                    onCheckedChange={(v) =>
                      setSelected(v ? data.map((r) => r.id) : [])
                    }
                  />
                </TableHead>
                <TableHead>{t("name")}</TableHead>
                <TableHead>{t("status")}</TableHead>
                <TableHead>{t("owner")}</TableHead>
                <TableHead>{t("budget")}</TableHead>
                <TableHead>{t("date")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map(({ original: r }) => (
                <TableRow key={r.id}>
                  <TableCell>
                    <Checkbox
                      aria-label={r.name}
                      checked={selected.includes(r.id)}
                      onCheckedChange={(v) =>
                        setSelected(
                          v
                            ? [...selected, r.id]
                            : selected.filter((id) => id !== r.id),
                        )
                      }
                    />
                  </TableCell>
                  <TableCell>
                    <Link to={`?pattern=recordDetail&record=${r.id}`}>
                      <strong>
                        {state === "long" ? r.name.repeat(8) : r.name}
                      </strong>
                    </Link>
                    <small>
                      {t("recordCodePrefix")}
                      {String(r.id + 1).padStart(3, "0")}
                      {t("metadataSeparator")} {t("detailsText")}
                    </small>
                  </TableCell>
                  <TableCell>
                    <Status value={r.status} />
                  </TableCell>
                  <TableCell>
                    <span className="sc-person">
                      {["AL", "林", "MK"][r.id % 3]}
                    </span>
                    {["Alex Lee", "林晓", "Mika Kim"][r.id % 3]}
                  </TableCell>
                  <TableCell className="sc-number">
                    {new Intl.NumberFormat(undefined, {
                      style: "currency",
                      currency: "CNY",
                      maximumFractionDigits: 0,
                    }).format(r.amount)}
                  </TableCell>
                  <TableCell>
                    {t("datePrefix")}
                    {String(10 + (r.id % 18)).padStart(2, "0")}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {!data.length && (
        <div className="sc-empty">
          <h2>{t("empty")}</h2>
          <Button
            variant="outline"
            onClick={() => {
              if (onReset) {
                onReset();
                return;
              }
              const next = new URLSearchParams(q);
              for (const key of ["q", "status", "group", "p"]) next.delete(key);
              setQ(next);
            }}
          >
            {t("clear")}
          </Button>
        </div>
      )}
      <footer className="sc-pagination">
        <span>{t("records", { count: data.length })}</span>
        <Button
          variant="outline"
          disabled={current === 1}
          onClick={() => {
            const n = new URLSearchParams(q);
            n.set("p", String(current - 1));
            setQ(n);
          }}
        >
          {t("previous")}
        </Button>
        <span>{t("page", { page: current, total: pages })}</span>
        <Button
          variant="outline"
          disabled={current === pages}
          onClick={() => {
            const n = new URLSearchParams(q);
            n.set("p", String(current + 1));
            setQ(n);
          }}
        >
          {t("next")}
        </Button>
      </footer>
    </>
  );
}
