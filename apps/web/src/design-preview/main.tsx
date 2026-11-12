import { createRoot } from "react-dom/client";
import { useState } from "react";
import {
  BookOpen,
  FileText,
  Inbox,
  Image,
  Layers,
  Search,
  Plus,
  ChevronDown,
  MoreHorizontal,
  LayoutGrid,
  List,
  Settings2,
  Clock,
  Check,
  X,
  PanelLeft,
  Bell,
  SlidersHorizontal,
  Copy,
  PenLine,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from "@/components/ui/dropdown-menu";
import { I18nextProvider } from "react-i18next";
import { createInstance } from "i18next";
import { copy as c, seed, type Article } from "./data";
import "@/styles/index.css";
import "./preview.css";
const language = createInstance();
void language.init({
  lng: "zh-CN",
  resources: { "zh-CN": { admin: { actions: { close: c.close } } } },
});
function StateBadge({ status }: { status: string }) {
  return (
    <Badge
      variant="secondary"
      className={`state-badge ${status === c.published ? "published" : status === c.review ? "review" : "draft"}`}
    >
      <span />
      {status}
    </Badge>
  );
}
function App() {
  const [rows, setRows] = useState(seed),
    [tab, setTab] = useState(c.all),
    [q, setQ] = useState(""),
    [category, setCategory] = useState("all"),
    [grid, setGrid] = useState(false),
    [selected, setSelected] = useState<number[]>([]),
    [detail, setDetail] = useState<Article | null>(null),
    [editor, setEditor] = useState<Article | null | undefined>(undefined),
    [title, setTitle] = useState(""),
    [summary, setSummary] = useState(""),
    [notice, setNotice] = useState(""),
    [reverse, setReverse] = useState(false),
    [showAuthor, setShowAuthor] = useState(true),
    [nav, setNav] = useState(false);
  const filtered = rows.filter(
    (r) =>
      (tab === c.all || r.status === tab) &&
      (category === "all" || r.category === category) &&
      `${r.title}${r.author}`.includes(q),
  );
  const visible = reverse ? [...filtered].reverse() : filtered;
  const openEditor = (row: Article | null) => {
    setEditor(row);
    setTitle(row?.title ?? "");
    setSummary(row?.summary ?? "");
  };
  const toggle = (id: number) =>
    setSelected((old) =>
      old.includes(id) ? old.filter((v) => v !== id) : [...old, id],
    );
  const actions = (row: Article) => (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon"
            aria-label={`${c.actions} ${row.title}`}
          />
        }
      >
        <MoreHorizontal size={18} />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => setDetail(row)}>
          <BookOpen />
          {c.view}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => openEditor(row)}>
          <PenLine />
          {c.edit}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setRows((old) => [
              { ...row, id: Date.now(), status: c.draft },
              ...old,
            ]);
            setNotice(c.saved);
          }}
        >
          <Copy />
          {c.duplicate}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
  return (
    <div className="design-app">
      <aside className={nav ? "design-sidebar is-open" : "design-sidebar"}>
        <a className="design-brand" href="/ui-preview.html">
          <span>
            <Layers size={21} />
          </span>
          {c.brand}
          <small>{c.demo}</small>
        </a>
        <div className="team-switch">
          <span className="team-logo">{c.teamInitial}</span>
          <div>
            <strong>{c.team}</strong>
            <small>{c.space}</small>
          </div>
          <ChevronDown size={14} />
        </div>
        <p className="nav-label">{c.space}</p>
        <nav>
          {c.nav.map((label, i) => {
            const Icon = [Layers, FileText, Inbox, Image][i];
            return (
              <Button
                key={label}
                variant="ghost"
                className={i === 1 ? "nav-item active" : "nav-item"}
                onClick={() => {
                  setTab(i === 2 ? c.review : c.all);
                  setNav(false);
                  if (i === 3) {
                    setGrid(true);
                    setNotice(c.note);
                  }
                }}
              >
                <Icon size={18} />
                {label}
                {i === 2 && (
                  <span className="nav-count">
                    {rows.filter((r) => r.status === c.review).length}
                  </span>
                )}
              </Button>
            );
          })}
        </nav>
        <div className="sidebar-bottom">
          <div className="quiet-note">
            <BookOpen size={19} />
            <strong>{c.queueHint}</strong>
            <span>{c.note}</span>
          </div>
          <Button
            variant="ghost"
            className="nav-item"
            onClick={() => setNotice(c.note)}
          >
            <Settings2 size={17} />
            {c.settings}
          </Button>
          <div className="profile">
            <span className="avatar">{c.account}</span>
            <div>
              <strong>{c.accountName}</strong>
              <small>{c.role}</small>
            </div>
            <ChevronDown size={14} />
          </div>
        </div>
      </aside>
      <div className="design-workspace">
        <header className="design-topbar">
          <div>
            <Button
              variant="ghost"
              size="icon"
              className="mobile-menu"
              aria-label={c.menu}
              onClick={() => setNav(!nav)}
            >
              <PanelLeft />
            </Button>
            <span>{c.space}</span>
            <span className="crumb-divider">{c.crumbSeparator}</span>
            <strong>{c.title}</strong>
          </div>
          <div>
            <Badge variant="outline" className="demo-badge">
              {c.preview}
            </Badge>
            <Button
              variant="ghost"
              size="icon"
              aria-label={c.queue}
              onClick={() => {
                setTab(c.review);
                setNotice(c.note);
              }}
            >
              <Bell size={18} />
            </Button>
            <span className="avatar small">{c.account}</span>
          </div>
        </header>
        <main>
          <div className="page-intro">
            <div>
              <p className="intro-context">
                <span />
                {c.space}
              </p>
              <h1>{c.title}</h1>
              <p>{c.subtitle}</p>
            </div>
            <Button className="create-button" onClick={() => openEditor(null)}>
              <Plus size={17} />
              {c.create}
            </Button>
          </div>
          <div className="workspace-overview">
            {[
              [c.countAll, rows.length],
              [c.countReview, rows.filter((r) => r.status === c.review).length],
              [
                c.countPublished,
                rows.filter((r) => r.status === c.published).length,
              ],
            ].map(([label, count], i) => (
              <button
                type="button"
                key={label}
                onClick={() => setTab([c.all, c.review, c.published][i])}
              >
                <span>{label}</span>
                <strong>
                  {count}
                  <small>{c.total}</small>
                </strong>
                {i === 1 && <span className="overview-dot" />}
              </button>
            ))}
          </div>
          <section className="content-surface" aria-label={c.title}>
            <div className="view-tabs" role="tablist" aria-label={c.status}>
              {[c.all, c.draft, c.review, c.published].map((label) => (
                <button
                  type="button"
                  role="tab"
                  aria-selected={tab === label}
                  key={label}
                  onClick={() => {
                    setTab(label);
                    setSelected([]);
                  }}
                >
                  {label}
                  <span>
                    {
                      rows.filter((r) => label === c.all || r.status === label)
                        .length
                    }
                  </span>
                </button>
              ))}
            </div>
            <div className="content-tools">
              <div className="article-search">
                <Search size={17} />
                <Input
                  aria-label={c.search}
                  placeholder={c.search}
                  value={q}
                  onChange={(e) => setQ(e.target.value)}
                />
              </div>
              <Select
                value={category}
                onValueChange={(v) => setCategory(String(v))}
              >
                <SelectTrigger aria-label={c.filter}>
                  <SlidersHorizontal size={15} />
                  <SelectValue>
                    {category === "all" ? c.category : category}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {["all", ...c.categories].map((v) => (
                    <SelectItem key={v} value={v}>
                      {v === "all" ? c.category : v}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <div className="tool-spacer" />
              <Button variant="ghost" onClick={() => setReverse(!reverse)}>
                <Clock size={15} />
                {reverse ? c.oldest : c.sort}
                <ChevronDown size={13} />
              </Button>
              <div className="view-switch">
                <Button
                  variant="ghost"
                  size="icon"
                  data-active={!grid}
                  aria-label={c.list}
                  onClick={() => setGrid(false)}
                >
                  <List size={17} />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  data-active={grid}
                  aria-label={c.grid}
                  onClick={() => setGrid(true)}
                >
                  <LayoutGrid size={16} />
                </Button>
              </div>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={c.columns}
                    />
                  }
                >
                  <Settings2 size={17} />
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => setShowAuthor(!showAuthor)}>
                    {showAuthor && <Check size={14} />} {c.authorColumn}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
            {selected.length > 0 && (
              <div className="selection-bar">
                <span>
                  {selected.length} {c.selected}
                </span>
                <Button variant="ghost" onClick={() => setSelected([])}>
                  {c.clear}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => {
                    setRows((old) =>
                      old.filter((r) => !selected.includes(r.id)),
                    );
                    setSelected([]);
                    setNotice(c.archived);
                  }}
                >
                  {c.archive}
                </Button>
                <small>{c.local}</small>
              </div>
            )}
            {visible.length === 0 ? (
              <div className="design-empty">
                <Search size={28} />
                <h2>{c.empty}</h2>
                <p>{c.emptyHint}</p>
                <Button
                  variant="outline"
                  onClick={() => {
                    setQ("");
                    setCategory("all");
                    setTab(c.all);
                  }}
                >
                  {c.reset}
                </Button>
              </div>
            ) : grid ? (
              <div className="article-grid">
                {visible.map((row) => (
                  <article key={row.id}>
                    <div>
                      <span className="category-text">{row.category}</span>
                      <StateBadge status={row.status} />
                    </div>
                    <button
                      type="button"
                      className="title-link"
                      onClick={() => setDetail(row)}
                    >
                      {row.title}
                    </button>
                    <p>{row.summary}</p>
                    <footer>
                      <span className="avatar small">{row.initials}</span>
                      <span>{row.author}</span>
                      <small>{row.date.slice(5)}</small>
                      {actions(row)}
                    </footer>
                  </article>
                ))}
              </div>
            ) : (
              <div className="article-table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th className="check-cell">
                        <Checkbox
                          aria-label={c.selectAll}
                          checked={
                            visible.length > 0 &&
                            visible.every((r) => selected.includes(r.id))
                          }
                          onCheckedChange={(checked) =>
                            setSelected(checked ? visible.map((r) => r.id) : [])
                          }
                        />
                      </th>
                      <th>{c.fields[0]}</th>
                      <th>{c.fields[1]}</th>
                      {showAuthor && <th>{c.fields[2]}</th>}
                      <th>{c.fields[3]}</th>
                      <th>
                        <span className="sr-only">{c.fields[4]}</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {visible.map((row) => (
                      <tr
                        key={row.id}
                        data-selected={selected.includes(row.id)}
                      >
                        <td className="check-cell">
                          <Checkbox
                            aria-label={`${c.select} ${row.title}`}
                            checked={selected.includes(row.id)}
                            onCheckedChange={() => toggle(row.id)}
                          />
                        </td>
                        <td className="article-title-cell">
                          <button
                            type="button"
                            className="title-link"
                            onClick={() => setDetail(row)}
                          >
                            {row.title}
                          </button>
                          <div>
                            <span>{row.category}</span>
                            <span className="meta-dot" />
                            <span>
                              {row.minutes} {c.minutes}
                            </span>
                          </div>
                        </td>
                        <td>
                          <StateBadge status={row.status} />
                        </td>
                        {showAuthor && (
                          <td>
                            <span className={`author author-${row.id % 3}`}>
                              <span className="avatar small">
                                {row.initials}
                              </span>
                              {row.author}
                            </span>
                          </td>
                        )}
                        <td className="date-cell">
                          {row.date}
                          <small>{row.time}</small>
                        </td>
                        <td>{actions(row)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <footer className="content-footer">
              <span>
                {visible.length} {c.total}
              </span>
              <div>
                <span>{c.page}</span>
                <Button variant="outline" disabled>
                  {c.previous}
                </Button>
                <Button variant="outline" disabled>
                  {c.next}
                </Button>
              </div>
            </footer>
          </section>
          <p className="preview-footnote">
            <span />
            {c.note}
          </p>
        </main>
      </div>
      <Dialog
        open={!!detail}
        onOpenChange={(open) => {
          if (!open) setDetail(null);
        }}
      >
        <DialogContent className="article-detail-dialog">
          <DialogHeader>
            <DialogTitle>{c.view}</DialogTitle>
            <DialogDescription>{c.note}</DialogDescription>
          </DialogHeader>
          {detail && (
            <>
              <div className="detail-heading">
                <span className="category-text">{detail.category}</span>
                <h2>{detail.title}</h2>
                <p>{detail.summary}</p>
                <div>
                  <StateBadge status={detail.status} />
                  <span className="author">
                    <span className="avatar small">{detail.initials}</span>
                    {detail.author}
                  </span>
                  <span>
                    {detail.minutes} {c.minutes}
                  </span>
                </div>
              </div>
              <div className="detail-body">
                <article>
                  {c.sections.map((heading, i) => (
                    <section key={heading}>
                      <h3>{heading}</h3>
                      <p>{i === 0 ? c.detailText : c.detailText2}</p>
                    </section>
                  ))}
                </article>
                <aside>
                  <h3>{c.info}</h3>
                  <dl>
                    <dt>{c.status}</dt>
                    <dd>{detail.status}</dd>
                    <dt>{c.author}</dt>
                    <dd>{detail.author}</dd>
                    <dt>{c.revision}</dt>
                    <dd>{c.revisionValue}</dd>
                  </dl>
                  <h3>{c.outline}</h3>
                  {c.sections.map((v) => (
                    <p key={v}>{v}</p>
                  ))}
                </aside>
              </div>
              <footer className="dialog-actions">
                <Button variant="outline" onClick={() => setDetail(null)}>
                  {c.close}
                </Button>
                <Button
                  onClick={() => {
                    openEditor(detail);
                    setDetail(null);
                  }}
                >
                  <PenLine size={15} />
                  {c.edit}
                </Button>
              </footer>
            </>
          )}
        </DialogContent>
      </Dialog>
      <Dialog
        open={editor !== undefined}
        onOpenChange={(open) => {
          if (!open) setEditor(undefined);
        }}
      >
        <DialogContent className="article-editor-dialog">
          <DialogHeader>
            <DialogTitle>{editor ? c.edit : c.create}</DialogTitle>
            <DialogDescription>{c.editorHint}</DialogDescription>
          </DialogHeader>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (!title.trim()) return;
              const row: Article = {
                ...(editor ?? seed[0]),
                id: editor?.id ?? Date.now(),
                title,
                summary,
                status: c.draft,
              };
              setRows((old) =>
                editor
                  ? old.map((r) => (r.id === editor.id ? row : r))
                  : [row, ...old],
              );
              setEditor(undefined);
              setNotice(c.saved);
            }}
          >
            <label>
              {c.name}
              <Input
                required
                value={title}
                placeholder={c.nameHint}
                onChange={(e) => setTitle(e.target.value)}
              />
            </label>
            <label>
              {c.summary}
              <Textarea
                rows={3}
                value={summary}
                onChange={(e) => setSummary(e.target.value)}
              />
            </label>
            <label>
              {c.body}
              <Textarea rows={8} defaultValue={editor ? c.detailText : ""} />
            </label>
            <footer className="dialog-actions">
              <span>{c.local}</span>
              <Button
                type="button"
                variant="outline"
                onClick={() => setEditor(undefined)}
              >
                {c.cancel}
              </Button>
              <Button type="submit">{c.save}</Button>
            </footer>
          </form>
        </DialogContent>
      </Dialog>
      {notice && (
        <div className="preview-toast" role="status">
          <Check size={17} />
          {notice}
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={c.close}
            onClick={() => setNotice("")}
          >
            <X size={14} />
          </Button>
        </div>
      )}
    </div>
  );
}
createRoot(document.getElementById("root")!).render(
  <I18nextProvider i18n={language}>
    <App />
  </I18nextProvider>,
);
