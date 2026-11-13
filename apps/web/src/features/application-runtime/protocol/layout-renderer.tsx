import {
  createContext,
  createElement,
  useContext,
  useCallback,
  useMemo,
  useSyncExternalStore,
  type CSSProperties,
} from "react";
import { evaluate, mapInput } from "../expression";
import { useTranslation } from "react-i18next";
import {
  PresentationContext,
  PresentationPermissions,
  type ComponentRegistry,
  type ProtocolEventOptions,
} from "./component-registry";
import { ErrorBanner, Loading } from "@/features/administration/shared";
import { resolveRef, type BindingState, type DataMap } from "./data-resolver";
import type {
  ComponentSpec,
  EventBinding,
  LayoutProps,
  LayoutSpec,
  ProtocolNode,
} from "./types";

export const BindingIndex = createContext<Record<string, BindingState>>({});
const BindingScope = createContext<Record<string, BindingState>>({});

export type ProtocolEventHandler = (
  events: EventBinding[],
  event: string,
  payload?: unknown,
  options?: ProtocolEventOptions,
) => void | Promise<boolean | void>;

export function ProtocolRenderer({
  node,
  dataMap,
  dataIndex,
  registry,
  onEvent,
  readOnly = false,
  layoutPath = "",
}: {
  node: ProtocolNode;
  dataMap: DataMap;
  dataIndex: Record<string, unknown>;
  registry: ComponentRegistry;
  onEvent: ProtocolEventHandler;
  readOnly?: boolean;
  layoutPath?: string;
}) {
  if (node.type === "layout" && node.layout) {
    return (
      <LayoutNode
        spec={node.layout}
        dataMap={dataMap}
        dataIndex={dataIndex}
        registry={registry}
        onEvent={onEvent}
        readOnly={readOnly}
        layoutPath={layoutPath}
      />
    );
  }
  if (node.type === "component" && node.component) {
    return (
      <ComponentNode
        spec={node.component}
        dataMap={dataMap}
        dataIndex={dataIndex}
        layoutPath={layoutPath}
        registry={registry}
        onEvent={onEvent}
        readOnly={readOnly}
      />
    );
  }
  return null;
}

function LayoutNode({
  spec,
  dataMap,
  dataIndex,
  registry,
  onEvent,
  readOnly,
  layoutPath,
  directChildren = false,
  unwrap = false,
}: {
  spec: LayoutSpec;
  dataMap: DataMap;
  dataIndex: Record<string, unknown>;
  registry: ComponentRegistry;
  onEvent: ProtocolEventHandler;
  readOnly: boolean;
  layoutPath: string;
  directChildren?: boolean;
  unwrap?: boolean;
}) {
  const childDataMap = { ...dataMap };
  const bindings = useContext(BindingIndex);
  const states = { ...useContext(BindingScope) };
  spec.data?.forEach((dataSpec, index) => {
    const key = `${layoutPath}:${index}:${dataSpec.id}`;
    childDataMap[dataSpec.id] = dataIndex[key] ?? null;
    delete states[dataSpec.id];
    if (bindings[key]) states[dataSpec.id] = bindings[key];
  });

  const style = useLayoutStyle(spec.props);
  const permissions = useContext(PresentationPermissions);
  const context = useContext(PresentationContext);
  if (
    spec.visible?.kind &&
    !evaluate(spec.visible, {
      data: childDataMap,
      context: { ...context, permissions },
    })
  )
    return null;
  const content = (
    <BindingScope.Provider value={states}>
      {spec.children?.map((child, index) => {
        if (child.type !== "layout" || !child.layout)
          return (
            <ProtocolRenderer
              key={child.component?.id ?? index}
              node={child}
              dataMap={childDataMap}
              dataIndex={dataIndex}
              registry={registry}
              onEvent={onEvent}
              readOnly={readOnly}
              layoutPath={layoutPath}
            />
          );
        const childPath = `${layoutPath}/${index}:${child.layout.id}`;
        return (
          <LayoutNode
            key={`${childPath}:${child.layout.id}`}
            spec={child.layout}
            unwrap={directChildren}
            dataMap={childDataMap}
            dataIndex={dataIndex}
            registry={registry}
            onEvent={onEvent}
            readOnly={readOnly}
            layoutPath={childPath}
          />
        );
      })}
    </BindingScope.Provider>
  );
  if (unwrap || directChildren) return content;
  return (
    <div
      data-layout-id={spec.id}
      data-layout-kind={spec.kind}
      className={buildClassName(spec)}
      style={style}
    >
      {content}
    </div>
  );
}

function ComponentNode({
  spec,
  dataMap,
  dataIndex,
  layoutPath,
  registry,
  onEvent,
  readOnly,
}: {
  spec: ComponentSpec;
  dataMap: DataMap;
  dataIndex: Record<string, unknown>;
  layoutPath: string;
  registry: ComponentRegistry;
  onEvent: ProtocolEventHandler;
  readOnly: boolean;
}) {
  const data = resolveRef(spec.data, dataMap);
  const state = useContext(BindingScope)[spec.data?.slice(1) ?? ""];
  const permissions = useContext(PresentationPermissions);
  const context = useContext(PresentationContext);
  const scope = {
    data: dataMap,
    record: data,
    context: { ...context, permissions },
  };
  if (spec.visible?.kind && !evaluate(spec.visible, scope)) return null;
  const props = { ...spec.props, ...mapInput(spec.bindings ?? [], scope) };
  const renderer = registry.resolve(spec.kind, spec.version);
  if (!renderer)
    return <MissingComponent kind={spec.kind} version={spec.version} />;
  if (state?.pending) return <Loading />;
  const content = createElement(renderer, {
    props,
    componentId: spec.id,
    dataRefreshing: state?.placeholder,
    events: spec.events?.map((event) => event.event),
    data,
    dataMap,
    readOnly,
    emit: (event, payload, options) =>
      onEvent(spec.events ?? [], event, payload, options),
    children: spec.content && (
      <LayoutNode
        spec={spec.content}
        directChildren={
          spec.kind === "carousel" ||
          spec.kind === "resizable" ||
          spec.kind === "avatar-group" ||
          (["alert", "empty", "item"].includes(spec.kind) &&
            props.sections === true) ||
          (spec.kind === "sidebar" && props.sections === true) ||
          (spec.kind === "navigation-menu" && props.group === true)
        }
        dataMap={dataMap}
        dataIndex={dataIndex}
        registry={registry}
        onEvent={onEvent}
        readOnly={readOnly}
        layoutPath={`${layoutPath}/component:${spec.id}`}
      />
    ),
  });
  return state?.error ? (
    <>
      <ErrorBanner error={state.error} onRetry={state.refetch} />
      {data != null && content}
    </>
  ) : (
    content
  );
}

function MissingComponent({
  kind,
  version,
}: {
  kind: string;
  version: string;
}) {
  const { t } = useTranslation("runtime");
  return (
    <div className="admin-error" role="alert">
      {t("protocol.componentMissing", { component: `${kind}@${version}` })}
    </div>
  );
}

const breakpoints: Array<[string, string]> = [
  ["xl", "(min-width: 1181px)"],
  ["lg", "(min-width: 1024px)"],
  ["md", "(min-width: 768px)"],
];

function useMediaQuery(query: string): boolean {
  const media = useMemo(
    () =>
      typeof window.matchMedia === "function"
        ? window.matchMedia(query)
        : undefined,
    [query],
  );
  const subscribe = useCallback(
    (onChange: () => void) => {
      if (!media) return () => undefined;
      media.addEventListener("change", onChange);
      return () => media.removeEventListener("change", onChange);
    },
    [media],
  );
  return useSyncExternalStore(
    subscribe,
    () => media?.matches ?? false,
    () => false,
  );
}

function useLayoutStyle(props: LayoutProps): CSSProperties {
  const wide = useMediaQuery(breakpoints[0][1]);
  const large = useMediaQuery(breakpoints[1][1]);
  const medium = useMediaQuery(breakpoints[2][1]);
  let effective = props;
  if (wide && props.responsive?.xl)
    effective = { ...props, ...props.responsive.xl };
  else if (large && props.responsive?.lg)
    effective = { ...props, ...props.responsive.lg };
  else if (medium && props.responsive?.md)
    effective = { ...props, ...props.responsive.md };
  else if (!medium && props.responsive?.sm)
    effective = { ...props, ...props.responsive.sm };
  return layoutToStyle(effective);
}

function layoutToStyle(props: LayoutProps): CSSProperties {
  const style: CSSProperties = {};
  if (props.columns) style.gridTemplateColumns = props.columns;
  if (props.rows) style.gridTemplateRows = props.rows;
  if (props.gap) style.gap = props.gap;
  if (props.areas) style.gridTemplateAreas = props.areas;
  if (props.justify) style.justifyItems = props.justify;
  if (props.align) style.alignItems = props.align;
  if (props.padding) style.padding = props.padding;
  if (props.width === "full") style.width = "100%";
  if (props.width === "reading") style.maxWidth = "72rem";
  if (props.width === "form") style.maxWidth = "30rem";
  return style;
}

function buildClassName(spec: LayoutSpec): string {
  const classes = ["protocol-layout", `protocol-${spec.kind}`];
  if (spec.props.className) classes.push(spec.props.className);
  if (spec.props.width && spec.props.width !== "auto")
    classes.push(`protocol-width-${spec.props.width}`);
  return classes.join(" ");
}
