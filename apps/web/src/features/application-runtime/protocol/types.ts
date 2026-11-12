// ApplicationRuntime Schema Protocol v3 uses the contract generated from the
// Go source; the renderer intentionally owns no duplicate DTO shape.
import type {
  RuntimeApplicationV3,
  RuntimeCapabilityRef,
  RuntimeComponentSpec,
  RuntimeDataSpec,
  RuntimeEventBinding,
  RuntimeExpression,
  RuntimeLayoutProps,
  RuntimeLayoutSpec,
  RuntimeMapping,
  RuntimeNode,
  RuntimePageV3,
  RuntimeNavigation,
  RuntimeProvider,
} from "@/api/generated/runtime";

export type ApplicationV3 = RuntimeApplicationV3;
export type NodeType = RuntimeNode["type"];
export type LayoutSpec = RuntimeLayoutSpec;
export type LayoutProps = RuntimeLayoutProps;
export type DataSpec = RuntimeDataSpec;
export type ComponentSpec = RuntimeComponentSpec;
export type EventBinding = RuntimeEventBinding;
export type Expression = RuntimeExpression;
export type Mapping = RuntimeMapping;
export type CapabilityRef = RuntimeCapabilityRef;
export type ProtocolProvider = RuntimeProvider;
export type PageV3 = RuntimePageV3;
export type ProtocolRecord = Record<string, unknown>;

export interface ProtocolTableColumn {
  name: string;
  labelKey?: string;
  display?: string;
  width?: number;
  className?: string;
}

export type ProtocolNode = RuntimeNode;
export type ProtocolNavigation = RuntimeNavigation;
export type EventAction = RuntimeEventBinding["action"];
export type LayoutKind = RuntimeLayoutSpec["kind"];
export type LayoutWidth = NonNullable<RuntimeLayoutProps["width"]>;
