import { useState } from "react";
import { ComponentRegistry } from "@/features/application-runtime/protocol/component-registry";
import {
  CardComponent,
  ContentFields,
  ContentTable,
  InlineInput,
} from "@/features/application-runtime/protocol/content-components";
import { ItemListComponent } from "@/features/application-runtime/protocol/item-list";
import { ProtocolRenderer } from "@/features/application-runtime/protocol/layout-renderer";
import type { ProtocolNode } from "@/features/application-runtime/protocol/types";
import "@/features/application-runtime/protocol/protocol.css";
import "@/features/ai/ai.css";

const registry = new ComponentRegistry()
  .register("card", "1.0", CardComponent)
  .register("content-fields", "1.0", ContentFields)
  .register("content-table", "1.0", ContentTable)
  .register("inline-input", "1.0", InlineInput)
  .register("item-list", "1.0", ItemListComponent);

function block(
  id: string,
  kind: string,
  props: Record<string, unknown>,
  data?: string,
): ProtocolNode {
  return {
    type: "layout",
    layout: {
      id: id + "-layout",
      kind: "grid",
      props: {},
      children: [
        {
          type: "component",
          component: { id, kind, version: "1.0", props, data },
        },
      ],
    },
  };
}

const node: ProtocolNode = {
  type: "layout",
  layout: {
    id: "composition-preview",
    kind: "grid",
    props: {},
    children: [
      {
        type: "component",
        component: {
          id: "composition-card",
          kind: "card",
          version: "1.0",
          props: {
            titleKey: "showcase:title",
            descriptionKey: "showcase:mock",
          },
          content: {
            id: "composition-content",
            kind: "grid",
            props: { padding: "16px", gap: "12px" },
            children: [
              block(
                "milestones",
                "item-list",
                {
                  ordered: true,
                  fields: [{ path: "labelKey", labelPrefix: "showcase:" }],
                  tail: [
                    {
                      path: "status",
                      labelPrefix: "showcase:",
                      className: "ai-badge",
                    },
                  ],
                },
                "$milestones",
              ),
              block("add-item", "inline-input", {
                inputId: "preview-item",
                labelKey: "showcase:item",
                submitKey: "showcase:new",
                placeholderKey: "showcase:name",
                pendingKey: "showcase:running",
                buttonClassName: "ai-button",
              }),
              block(
                "items",
                "content-table",
                {
                  emptyKey: "showcase:empty",
                  searchKey: "showcase:search",
                  searchFields: ["name"],
                  columns: [
                    { labelKey: "showcase:name", fields: [{ path: "name" }] },
                    {
                      labelKey: "showcase:status",
                      fields: [{ path: "status", labelPrefix: "showcase:" }],
                    },
                  ],
                },
                "$items",
              ),
            ],
          },
        },
      },
    ],
  },
};

const milestones = [
  { id: 1, labelKey: "basics", status: "done" },
  { id: 2, labelKey: "scope", status: "active" },
  { id: 3, labelKey: "confirm", status: "review" },
];

export function CardComposition() {
  const [items, setItems] = useState<
    { id: number; name: string; status: string }[]
  >([]);
  return (
    <ProtocolRenderer
      node={node}
      dataMap={{ items, milestones }}
      dataIndex={{}}
      registry={registry}
      onEvent={(_, event, payload) => {
        if (
          event !== "submit" ||
          !payload ||
          typeof payload !== "object" ||
          !("value" in payload) ||
          typeof payload.value !== "string"
        )
          return;
        const name = payload.value;
        setItems((current) => [
          ...current,
          { id: current.length + 1, name, status: "active" },
        ]);
      }}
    />
  );
}
