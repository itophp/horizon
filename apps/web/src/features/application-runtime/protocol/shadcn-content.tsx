import { CardFooter } from "@/components/ui/card";
import { AlertAction } from "@/components/ui/alert";
import { EmptyMedia } from "@/components/ui/empty";
import { ItemActions, ItemMedia, ItemFooter } from "@/components/ui/item";
import { Button } from "@/components/ui/button";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AspectRatio } from "@/components/ui/aspect-ratio";
import {
  Avatar,
  AvatarFallback,
  AvatarGroup,
  AvatarImage,
} from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  Carousel,
  type CarouselApi,
  CarouselContent,
  CarouselItem,
  CarouselNext,
  CarouselPrevious,
} from "@/components/ui/carousel";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from "@/components/ui/empty";
import {
  Item,
  ItemContent,
  ItemDescription,
  ItemTitle,
} from "@/components/ui/item";
import { Kbd } from "@/components/ui/kbd";
import { Label } from "@/components/ui/label";
import {
  Progress,
  ProgressLabel,
  ProgressValue as ProgressDisplay,
} from "@/components/ui/progress";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Fragment, useEffect, useRef, useState } from "react";
import type { ComponentRenderer } from "./component-registry";
import { useBoundValue, useUI } from "./shadcn-props";

export const contentRenderers = {
  accordion: function AccordionSection(p) {
    const { label, event } = useUI(p);
    const group = p.props.group === true;
    const initial = Array.isArray(p.props.expanded)
      ? p.props.expanded.filter(
          (item): item is string => typeof item === "string",
        )
      : p.props.open === true && !group
        ? ["section"]
        : [];
    const [expanded, setExpanded] = useBoundValue(JSON.stringify(initial));
    const values = JSON.parse(expanded) as string[];
    return (
      <Accordion
        aria-label={label}
        multiple={group && p.props.multiple === true}
        value={values}
        onValueChange={(next: string[]) => {
          setExpanded(JSON.stringify(next));
          event("change", next);
        }}
      >
        {group ? (
          p.children
        ) : (
          <AccordionItem value="section">
            <AccordionTrigger>{label}</AccordionTrigger>
            <AccordionContent>{p.children}</AccordionContent>
          </AccordionItem>
        )}
      </Accordion>
    );
  },
  "accordion-item": function AccordionSectionItem(p) {
    const { label, string } = useUI(p);
    return (
      <AccordionItem
        value={string("value")}
        disabled={p.props.disabled === true}
      >
        <AccordionTrigger>{label}</AccordionTrigger>
        <AccordionContent>{p.children}</AccordionContent>
      </AccordionItem>
    );
  },
  alert: function Notice(p) {
    const { title, description } = useUI(p);
    return (
      <Alert
        variant={p.props.variant === "destructive" ? "destructive" : "default"}
      >
        <AlertTitle>{title}</AlertTitle>
        <AlertDescription>
          {description}
          {p.props.sections !== true && p.children}
        </AlertDescription>
        {p.props.sections === true && p.children}
      </Alert>
    );
  },
  "aspect-ratio": function Ratio(p) {
    const { number } = useUI(p);
    return (
      <AspectRatio ratio={Math.max(0.1, number("ratio") ?? 16 / 9)}>
        {p.children}
      </AspectRatio>
    );
  },
  avatar: function ProfileAvatar(p) {
    const { label, string } = useUI(p);
    const image = string("src");
    // Same-origin paths keep image requests within the deployed application.
    const src =
      image && /^\/(?![/\\])[^\s\\]*$/.test(image) ? image : undefined;
    const size =
      p.props.size === "sm" || p.props.size === "lg" ? p.props.size : "default";
    return (
      <Avatar role="img" aria-label={label} size={size}>
        {src && <AvatarImage src={src} alt="" />}
        <AvatarFallback>
          {(string("initials") ?? label ?? "").slice(0, 3)}
        </AvatarFallback>
      </Avatar>
    );
  },
  "avatar-group": function Profiles(p) {
    const { label } = useUI(p);
    return (
      <AvatarGroup role="group" aria-label={label}>
        {p.children}
      </AvatarGroup>
    );
  },
  badge: function StatusBadge(p) {
    const { label } = useUI(p);
    const variant = p.props.variant;
    return (
      <Badge
        variant={
          variant === "default" ||
          variant === "destructive" ||
          variant === "outline" ||
          variant === "ghost" ||
          variant === "link"
            ? variant
            : "secondary"
        }
      >
        {label}
      </Badge>
    );
  },
  breadcrumb: function BreadcrumbTrail(p) {
    const { options, label, event, disabled } = useUI(p);
    return (
      <Breadcrumb aria-label={label}>
        <BreadcrumbList>
          {options.map(({ value, label }, index) => (
            <Fragment key={value}>
              {index > 0 && <BreadcrumbSeparator />}
              <BreadcrumbItem>
                {index === options.length - 1 ? (
                  <BreadcrumbPage>{label}</BreadcrumbPage>
                ) : (
                  <Button
                    type="button"
                    variant="link"
                    disabled={disabled}
                    onClick={() => event("select", value)}
                  >
                    {label}
                  </Button>
                )}
              </BreadcrumbItem>
            </Fragment>
          ))}
        </BreadcrumbList>
      </Breadcrumb>
    );
  },
  "button-group": (p) => (
    <ButtonGroup
      orientation={
        p.props.orientation === "vertical" ? "vertical" : "horizontal"
      }
    >
      {p.children}
    </ButtonGroup>
  ),
  carousel: function Slides(p) {
    const { label, t, number } = useUI(p);
    const [api, setApi] = useState<CarouselApi>();
    const [paused, setPaused] = useState(false);
    const [hovered, setHovered] = useState(false);
    const [focused, setFocused] = useState(false);
    const emit = useRef(p.emit);
    useEffect(() => {
      emit.current = p.emit;
    }, [p.emit]);
    const start = Math.max(
      0,
      Math.min(499, Math.trunc(number("startIndex") ?? 0)),
    );
    const autoplay = number("autoplayMs");
    const enabled = autoplay !== undefined && autoplay > 0 && !p.readOnly;
    useEffect(() => {
      if (!api || p.readOnly) return;
      const select = () => {
        void emit.current("change", api.selectedScrollSnap());
      };
      api.on("select", select);
      return () => {
        api.off("select", select);
      };
    }, [api, p.readOnly]);
    useEffect(() => {
      if (
        !api ||
        !enabled ||
        autoplay === undefined ||
        paused ||
        hovered ||
        focused
      )
        return;
      if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches)
        return;
      const timer = window.setInterval(
        () => {
          if (document.hidden) return;
          if (api.canScrollNext()) api.scrollNext();
        },
        Math.max(1000, Math.min(60000, autoplay)),
      );
      return () => window.clearInterval(timer);
    }, [api, autoplay, enabled, focused, hovered, paused]);
    return (
      <Carousel
        aria-label={label}
        aria-roledescription={t("components.carousel")}
        orientation={
          p.props.orientation === "vertical" ? "vertical" : "horizontal"
        }
        opts={{ loop: p.props.loop === true, startIndex: start }}
        setApi={setApi}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        onFocus={() => setFocused(true)}
        onBlur={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget))
            setFocused(false);
        }}
        className={
          p.props.orientation === "vertical"
            ? "my-10 [&_[data-slot=carousel-content]]:max-h-80"
            : "mx-10"
        }
      >
        {enabled && (
          <Button variant="ghost" size="sm" onClick={() => setPaused(!paused)}>
            {paused
              ? t("components.playCarousel")
              : t("components.pauseCarousel")}
          </Button>
        )}
        <CarouselContent>{p.children}</CarouselContent>
        {p.props.controls !== false && (
          <>
            <CarouselPrevious aria-label={t("components.previousSlide")} />
            <CarouselNext aria-label={t("components.nextSlide")} />
          </>
        )}
      </Carousel>
    );
  },
  "carousel-slide": function Slide(p) {
    const { t } = useUI(p);
    return (
      <CarouselItem aria-roledescription={t("components.slide")}>
        {p.children}
      </CarouselItem>
    );
  },
  collapsible: function Disclosure(p) {
    const { label, event } = useUI(p);
    const [open, setOpen] = useBoundValue(p.props.open === true);
    return (
      <Collapsible
        open={open}
        disabled={p.props.disabled === true}
        onOpenChange={(next) => {
          setOpen(next);
          event("openChange", next);
        }}
      >
        <CollapsibleTrigger>{label}</CollapsibleTrigger>
        <CollapsibleContent>{p.children}</CollapsibleContent>
      </Collapsible>
    );
  },
  empty: function EmptyState(p) {
    const { title, description } = useUI(p);
    return (
      <Empty>
        {p.props.sections === true && p.children}
        <EmptyHeader>
          <EmptyTitle>{title}</EmptyTitle>
          <EmptyDescription>{description}</EmptyDescription>
        </EmptyHeader>
        {p.props.sections !== true && p.children && (
          <EmptyContent>{p.children}</EmptyContent>
        )}
      </Empty>
    );
  },
  item: function ListItem(p) {
    const { title, description } = useUI(p);
    return (
      <Item variant="outline">
        <ItemContent>
          <ItemTitle>{title}</ItemTitle>
          <ItemDescription>{description}</ItemDescription>
          {p.props.sections !== true && p.children}
        </ItemContent>
        {p.props.sections === true && p.children}
      </Item>
    );
  },
  kbd: function KeyboardHint(p) {
    const { label } = useUI(p);
    return <Kbd>{label}</Kbd>;
  },
  label: function ControlLabel(p) {
    const { label, string } = useUI(p);
    return <Label htmlFor={string("for")}>{label}</Label>;
  },
  progress: function ProgressValue(p) {
    const { label, value, number, i18n } = useUI(p);
    const min = number("min") ?? 0;
    const max = Math.max(min + 1, number("max") ?? 100);
    const current =
      typeof value === "number" && Number.isFinite(value)
        ? Math.max(min, Math.min(max, value))
        : null;
    const numeric = p.props.format === "number";
    const formatted =
      current === null
        ? undefined
        : new Intl.NumberFormat(i18n.resolvedLanguage, {
            style: numeric ? "decimal" : "percent",
            maximumFractionDigits: 1,
          }).format(numeric ? current : (current - min) / (max - min));
    return (
      <Progress
        value={current}
        min={min}
        max={max}
        getAriaValueText={() => formatted ?? ""}
      >
        <ProgressLabel>{label}</ProgressLabel>
        {p.props.showValue === true && formatted && (
          <ProgressDisplay>{() => formatted}</ProgressDisplay>
        )}
      </Progress>
    );
  },
  resizable: (p) => (
    <ResizablePanelGroup
      orientation={
        p.props.orientation === "vertical" ? "vertical" : "horizontal"
      }
    >
      {p.children}
    </ResizablePanelGroup>
  ),
  "resizable-panel": function Panel(p) {
    const { number } = useUI(p);
    const min = Math.max(0, Math.min(100, number("minSize") ?? 0));
    const max = Math.max(min, Math.min(100, number("maxSize") ?? 100));
    const initial = number("defaultSize");
    return (
      <ResizablePanel
        minSize={`${min}%`}
        maxSize={`${max}%`}
        defaultSize={
          initial === undefined
            ? undefined
            : `${Math.max(min, Math.min(max, initial))}%`
        }
        disabled={p.props.disabled === true}
      >
        {p.children}
      </ResizablePanel>
    );
  },
  "resizable-handle": () => <ResizableHandle withHandle />,
  "scroll-area": function ScrollRegion(p) {
    const { number, label } = useUI(p);
    return (
      <ScrollArea
        aria-label={label}
        axes={
          p.props.axes === "horizontal" || p.props.axes === "both"
            ? p.props.axes
            : "vertical"
        }
        keepScrollbars={p.props.keepScrollbars === true}
        style={{
          height: Math.max(40, Math.min(1600, number("height") ?? 280)),
        }}
      >
        {p.children}
      </ScrollArea>
    );
  },
  separator: (p) => (
    <Separator
      orientation={
        p.props.orientation === "vertical" ? "vertical" : "horizontal"
      }
    />
  ),
  skeleton: (p) => (
    <Skeleton
      className={
        p.props.shape === "circle"
          ? "size-10 rounded-full"
          : p.props.shape === "block"
            ? "h-24 w-full"
            : "h-6 w-full"
      }
      aria-hidden
    />
  ),
  spinner: function LoadingIndicator(p) {
    const { label, t } = useUI(p);
    return <Spinner aria-label={label ?? t("components.loading")} />;
  },
  table: function PlainTable(p) {
    const { label, options } = useUI(p);
    const rows = Array.isArray(p.data) ? p.data : [];
    return (
      <Table>
        {label && <TableCaption>{label}</TableCaption>}
        <TableHeader>
          <TableRow>
            {options.map(({ value, label }) => (
              <TableHead key={value}>{label}</TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row: unknown, index) => (
            <TableRow key={index}>
              {options.map(({ value }) => {
                const cell: unknown =
                  typeof row === "object" && row && Object.hasOwn(row, value)
                    ? (row as Record<string, unknown>)[value]
                    : null;
                return (
                  <TableCell key={value}>
                    {typeof cell === "string" || typeof cell === "number"
                      ? cell
                      : null}
                  </TableCell>
                );
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    );
  },
  "card-footer": (p) => <CardFooter>{p.children}</CardFooter>,
  "alert-action": (p) => <AlertAction>{p.children}</AlertAction>,
  "empty-media": (p) => (
    <EmptyMedia variant={p.props.variant === "icon" ? "icon" : "default"}>
      {p.children}
    </EmptyMedia>
  ),
  "item-media": (p) => (
    <ItemMedia
      variant={
        p.props.variant === "icon" || p.props.variant === "image"
          ? p.props.variant
          : "default"
      }
    >
      {p.children}
    </ItemMedia>
  ),
  "item-actions": (p) => <ItemActions>{p.children}</ItemActions>,
  "item-footer": (p) => <ItemFooter>{p.children}</ItemFooter>,
  typography: function Typeset(p) {
    const { label, text } = useUI(p);
    const variants = {
      h1: ["h1", "text-3xl font-semibold"],
      h2: ["h2", "text-xl font-semibold"],
      h3: ["h3", "text-lg font-semibold"],
      h4: ["h4", "font-semibold"],
      h5: ["h5", "text-sm font-semibold"],
      h6: ["h6", "text-xs font-semibold"],
      heading: ["h2", "text-xl font-semibold"],
      caption: ["small", "text-muted-foreground"],
      quote: ["blockquote", "border-s-2 ps-4 italic"],
      code: ["code", "rounded bg-muted px-1.5 py-0.5 font-mono text-sm"],
      paragraph: ["p", "text-sm"],
    } as const;
    const variant = p.props.variant;
    if (variant === "ul" || variant === "ol") {
      const Tag = variant;
      return (
        <Tag
          aria-label={label}
          className={variant === "ol" ? "ms-5 list-decimal" : "ms-5 list-disc"}
        >
          {(Array.isArray(p.props.itemKeys)
            ? p.props.itemKeys.slice(0, 100)
            : []
          ).map((key: unknown, index) => (
            <li key={index}>{text(key)}</li>
          ))}
        </Tag>
      );
    }
    const [Tag, className] =
      typeof variant === "string" && Object.hasOwn(variants, variant)
        ? variants[variant as keyof typeof variants]
        : variants.paragraph;
    return <Tag className={className}>{label}</Tag>;
  },
} satisfies Record<string, ComponentRenderer>;
