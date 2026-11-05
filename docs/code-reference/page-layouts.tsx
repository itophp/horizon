import type { ReactNode } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { PageHeading } from "@/features/administration/shared";

export function TemplatePage({
  title,
  description,
  actions,
  children,
}: {
  title: string;
  description: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="min-w-0 space-y-6">
      <PageHeading title={title} subtitle={description}>
        {actions}
      </PageHeading>
      {children}
    </section>
  );
}

export function TemplatePanel({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle>
          <h2>{title}</h2>
        </CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

export function SplitLayout({
  main,
  aside,
  balanced = false,
}: {
  main: ReactNode;
  aside: ReactNode;
  balanced?: boolean;
}) {
  return (
    <div
      className={
        balanced
          ? "grid min-w-0 gap-6 xl:grid-cols-2"
          : "grid min-w-0 gap-6 xl:grid-cols-[minmax(0,1fr)_300px]"
      }
    >
      <div className="min-w-0 space-y-6">{main}</div>
      <div className="min-w-0 space-y-6">{aside}</div>
    </div>
  );
}

export function FocusLayout({ children }: { children: ReactNode }) {
  return <div className="mx-auto w-full max-w-3xl space-y-6">{children}</div>;
}

export function MetricCard({
  label,
  value,
  description,
}: {
  label: string;
  value: ReactNode;
  description: string;
}) {
  return (
    <Card size="sm">
      <CardHeader>
        <CardDescription>{label}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2">
        <p className="text-3xl font-semibold tabular-nums">{value}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </CardContent>
    </Card>
  );
}
