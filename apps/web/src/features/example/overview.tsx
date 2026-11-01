import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { tableFeatures, useTable, type ColumnDef } from "@tanstack/react-table";
import { Activity, ArrowRight, CheckCircle2, Globe2 } from "lucide-react";
import { ApiError } from "@/api/client/request";
import { useSystemEcho, useSystemHealth } from "@/api/generated/baseline";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  formatDate,
  formatNumber,
  formatCurrency,
  exampleInstant,
} from "@/lib/format";
const features = tableFeatures({});
type Row = { component: string; purpose: string };
function Infrastructure() {
  const { t } = useTranslation();
  const data = useMemo(
    () => [
      { component: t("table.react"), purpose: t("table.frontend") },
      { component: t("table.go"), purpose: t("table.backend") },
      { component: t("table.postgres"), purpose: t("table.database") },
    ],
    [t],
  );
  const columns = useMemo<ColumnDef<typeof features, Row>[]>(
    () => [
      { accessorKey: "component", header: t("table.component") },
      { accessorKey: "purpose", header: t("table.purpose") },
    ],
    [t],
  );
  const table = useTable({ features, data, columns });
  return (
    <Card className="infrastructure">
      <CardHeader>
        <CardTitle>{t("table.title")}</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((group) => (
              <TableRow key={group.id}>
                {group.headers.map((header) => (
                  <TableHead key={header.id}>
                    <table.FlexRender header={header} />
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows.map((row) => (
              <TableRow key={row.id}>
                {row.getAllCells().map((cell) => (
                  <TableCell key={cell.id}>
                    <table.FlexRender cell={cell} />
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
function Echo() {
  const { t, i18n } = useTranslation();
  const schema = z.object({
    name: z
      .string()
      .trim()
      .min(1, { message: "name" })
      .max(80, { message: "name" }),
  });
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { name: "" },
  });
  const [name, setName] = useState("");
  const result = useSystemEcho(
    { name },
    { query: { enabled: !!name, retry: false } },
  );
  const errorCode =
    result.error instanceof ApiError ? result.error.code : "internal";
  return (
    <Card>
      <CardHeader>
        <div className="card-icon">
          <ArrowRight size={19} />
        </div>
        <CardTitle>{t("echo.title")}</CardTitle>
        <CardDescription>{t("echo.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form
          onSubmit={handleSubmit((values) => setName(values.name))}
          noValidate
        >
          <label htmlFor="echo-name">{t("echo.name")}</label>
          <div className="input-row">
            <Input
              id="echo-name"
              placeholder={t("echo.placeholder")}
              aria-invalid={!!errors.name}
              aria-describedby={errors.name ? "name-error" : undefined}
              {...register("name")}
            />
            <Button type="submit" disabled={result.isFetching}>
              {result.isFetching ? t("loading") : t("echo.submit")}
              <ArrowRight size={15} />
            </Button>
          </div>
          {errors.name && (
            <p id="name-error" role="alert" className="error">
              {t("validation:name")}
            </p>
          )}
        </form>
        <div aria-live="polite">
          {result.isError && (
            <p role="alert" className="error">
              {t(`validation:errors.${errorCode}`)}
            </p>
          )}
          {result.data && (
            <div className="echo-result">
              <CheckCircle2 size={18} />
              <div>
                <strong>
                  {t("echo.result", { name: result.data.data.name })}
                </strong>
                <p>
                  {t("echo.timestamp")}
                  <time dateTime={result.data.data.timestamp}>
                    {formatDate(
                      result.data.data.timestamp,
                      i18n.resolvedLanguage ?? "en",
                    )}
                  </time>
                </p>
              </div>
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
export function Overview() {
  const { t, i18n } = useTranslation();
  const locale = i18n.resolvedLanguage ?? "en";
  const health = useSystemHealth();
  return (
    <>
      <section className="page-intro">
        <span className="eyebrow">{t("framework")}</span>
        <h1>{t("title")}</h1>
        <p>{t("subtitle")}</p>
      </section>
      <section className="health-strip" aria-live="polite">
        <span className="health-icon">
          <Activity size={21} />
        </span>
        <div>
          <h2>{t("api.title")}</h2>
          <p>{t("api.description")}</p>
        </div>
        <span
          className={health.isSuccess ? "status-badge" : "status-badge muted"}
        >
          {health.isPending
            ? t("loading")
            : health.isSuccess
              ? t("api.healthy")
              : t("api.failed")}
        </span>
        {health.isError && (
          <Button
            variant="outline"
            onClick={() => {
              void health.refetch();
            }}
          >
            {t("actions.retry")}
          </Button>
        )}
      </section>
      <div className="demo-grid">
        <Echo />
        <Card>
          <CardHeader>
            <div className="card-icon">
              <Globe2 size={19} />
            </div>
            <CardTitle>{t("formats.title")}</CardTitle>
            <CardDescription>{t("formats.description")}</CardDescription>
          </CardHeader>
          <CardContent>
            <dl className="format-list">
              <div>
                <dt>{t("formats.date")}</dt>
                <dd>{formatDate(exampleInstant, locale)}</dd>
              </div>
              <div>
                <dt>{t("formats.number")}</dt>
                <dd>{formatNumber(1234567.89, locale)}</dd>
              </div>
              <div>
                <dt>{t("formats.currency")}</dt>
                <dd>{formatCurrency(1234.5, locale)}</dd>
              </div>
            </dl>
            <p className="example-count">{t("formats.count", { count: 3 })}</p>
          </CardContent>
        </Card>
      </div>
      <Infrastructure />
    </>
  );
}
