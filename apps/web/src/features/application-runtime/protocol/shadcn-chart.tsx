import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  Pie,
  PieChart,
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  RadialBar,
  RadialBarChart,
  XAxis,
  YAxis,
} from "recharts";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ComponentRendererProps } from "./component-registry";
import { useUI } from "./shadcn-props";

export function RuntimeChart(p: ComponentRendererProps) {
  const { title, label, string, t, i18n } = useUI(p);
  const labelField = string("labelField") ?? "label";
  const variant = string("variant");
  const circular =
    variant === "pie" || variant === "donut" || variant === "radial-bar";
  const seen = new Set<string>();
  const configured = Array.isArray(p.props.series)
    ? p.props.series.slice(0, 10).flatMap((item: unknown) => {
        if (
          !item ||
          typeof item !== "object" ||
          !("field" in item) ||
          typeof item.field !== "string" ||
          !item.field ||
          seen.has(item.field) ||
          !("labelKey" in item) ||
          typeof item.labelKey !== "string"
        )
          return [];
        seen.add(item.field);
        return [{ field: item.field, label: t(item.labelKey) }];
      })
    : [];
  const series = (
    configured.length
      ? circular
        ? configured.slice(0, 1)
        : configured
      : [{ field: string("valueField") ?? "value", label }]
  ).map((item, index) => ({
    ...item,
    key: `series${index}`,
    color: `var(--chart-${(index % 5) + 1})`,
  }));
  const rows = Array.isArray(p.data)
    ? p.data.slice(0, 500).flatMap((row: unknown) => {
        if (!row || typeof row !== "object" || !Object.hasOwn(row, labelField))
          return [];
        const record = row as Record<string, unknown>;
        const name = record[labelField];
        if (typeof name !== "string") return [];
        const values = series.map(({ field }) => {
          const value = Object.hasOwn(record, field)
            ? record[field]
            : undefined;
          return typeof value === "number" &&
            Number.isFinite(value) &&
            (!circular || value > 0)
            ? value
            : null;
        });
        return values.some((value) => value !== null) ? [{ name, values }] : [];
      })
    : [];
  const data = rows.map(({ name, values }) => ({
    name,
    ...Object.fromEntries(series.map(({ key }, index) => [key, values[index]])),
  }));
  const config: ChartConfig = Object.fromEntries(
    series.map(({ key, label, color }) => [key, { label, color }]),
  );
  const axes = (
    <>
      <CartesianGrid vertical={false} />
      <XAxis dataKey="name" />
      <YAxis />
      <ChartTooltip content={<ChartTooltipContent />} />
      {(p.props.legend === true ||
        (p.props.legend !== false && series.length > 1)) && (
        <ChartLegend content={<ChartLegendContent />} />
      )}
    </>
  );
  const circularData = circular
    ? data.map((row, index) => ({
        ...row,
        fill: `var(--chart-${(index % 5) + 1})`,
      }))
    : [];
  const stackId = p.props.stacked === true ? "total" : undefined;
  const formatter = new Intl.NumberFormat(i18n.resolvedLanguage);
  return (
    <figure aria-label={title}>
      {title && <figcaption>{title}</figcaption>}
      {data.length === 0 ? (
        <p>{t("protocol.empty")}</p>
      ) : (
        <>
          <ChartContainer config={config} className="min-h-48 w-full">
            {variant === "line" ? (
              <LineChart accessibilityLayer data={data}>
                {axes}
                {series.map(({ key }) => (
                  <Line
                    key={key}
                    dataKey={key}
                    stroke={`var(--color-${key})`}
                  />
                ))}
              </LineChart>
            ) : variant === "area" ? (
              <AreaChart accessibilityLayer data={data}>
                {axes}
                {series.map(({ key }) => (
                  <Area
                    key={key}
                    dataKey={key}
                    stackId={stackId}
                    stroke={`var(--color-${key})`}
                    fill={`var(--color-${key})`}
                    fillOpacity={0.3}
                  />
                ))}
              </AreaChart>
            ) : variant === "radar" ? (
              <RadarChart accessibilityLayer data={data}>
                <PolarGrid />
                <PolarAngleAxis dataKey="name" />
                <PolarRadiusAxis />
                <ChartTooltip content={<ChartTooltipContent />} />
                {(p.props.legend === true ||
                  (p.props.legend !== false && series.length > 1)) && (
                  <ChartLegend content={<ChartLegendContent />} />
                )}
                {series.map(({ key }) => (
                  <Radar
                    key={key}
                    dataKey={key}
                    stroke={`var(--color-${key})`}
                    fill={`var(--color-${key})`}
                    fillOpacity={0.25}
                  />
                ))}
              </RadarChart>
            ) : variant === "radial-bar" ? (
              <RadialBarChart
                accessibilityLayer
                data={circularData}
                innerRadius="20%"
                outerRadius="85%"
              >
                <PolarAngleAxis type="number" tick={false} />
                <ChartTooltip content={<ChartTooltipContent hideLabel />} />
                <RadialBar dataKey={series[0].key} />
              </RadialBarChart>
            ) : circular ? (
              <PieChart accessibilityLayer>
                <ChartTooltip
                  content={<ChartTooltipContent nameKey="name" />}
                />
                <Pie
                  nameKey="name"
                  dataKey={series[0].key}
                  innerRadius={variant === "donut" ? "55%" : undefined}
                  data={circularData}
                />
              </PieChart>
            ) : (
              <BarChart accessibilityLayer data={data}>
                {axes}
                {series.map(({ key }) => (
                  <Bar
                    key={key}
                    dataKey={key}
                    stackId={stackId}
                    fill={`var(--color-${key})`}
                    radius={4}
                  />
                ))}
              </BarChart>
            )}
          </ChartContainer>
          <details>
            <summary>{t("components.chartData")}</summary>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("components.chartCategory")}</TableHead>
                  {series.map(({ key, label }) => (
                    <TableHead key={key}>{label}</TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map(({ name, values }, index) => (
                  <TableRow key={index}>
                    <TableCell>{name}</TableCell>
                    {series.map(({ key }, index) => (
                      <TableCell key={key}>
                        {values[index] === null
                          ? null
                          : formatter.format(values[index])}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </details>
        </>
      )}
    </figure>
  );
}
