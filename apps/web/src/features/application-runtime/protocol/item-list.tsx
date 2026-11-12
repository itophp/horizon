import { ArrowUpRight } from "lucide-react";
import { Fragment } from "react";
import { Link } from "react-router";
import { evaluate, pathValue } from "../expression";
import type { ComponentRendererProps } from "./component-registry";
import {
  ActionControl,
  contentIcons,
  isInternalHref,
  useContentFields,
} from "./content-components";

import type { Expression } from "./types";

type Field = Parameters<ReturnType<typeof useContentFields>["field"]>[0];
type Action = Parameters<typeof ActionControl>[0]["action"];

export function ItemListComponent({
  props,
  data,
  dataMap,
  readOnly,
  emit,
}: ComponentRendererProps) {
  const { t, field, scope, check } = useContentFields(dataMap);
  const source = props.items ?? data;
  const items: unknown[] = Array.isArray(source)
    ? source.filter((item: unknown) =>
        check(props.itemWhen as Expression | undefined, item),
      )
    : [];
  const fields = (props.fields ?? []) as Field[];
  const tail = (props.tail ?? []) as Field[];
  const actions = (props.actions ?? []) as Action[];
  const LeadingIcon =
    contentIcons[props.leadingIcon as keyof typeof contentIcons];
  const List = props.ordered === true ? "ol" : "ul";
  return (
    <>
      {typeof props.titleKey === "string" && <h3>{t(props.titleKey)}</h3>}
      {items.length ? (
        <List className={String(props.className ?? "")}>
          {items.map((item, index) => {
            const rowHref = props.rowHref as Expression | undefined;
            const href = rowHref?.kind
              ? evaluate(rowHref, scope(item))
              : undefined;
            const content = (
              <>
                {LeadingIcon && (
                  <span className="entity-symbol" aria-hidden>
                    <LeadingIcon size={15} />
                  </span>
                )}
                <div>
                  {fields.map((config, position) => (
                    <Fragment key={position}>{field(config, item)}</Fragment>
                  ))}
                </div>
                {tail.map((config, position) => (
                  <Fragment key={position}>{field(config, item)}</Fragment>
                ))}
              </>
            );
            return (
              <li
                key={String(
                  pathValue(item, String(props.keyField ?? "id")) ?? index,
                )}
                className={String(props.itemClassName ?? "")}
              >
                {isInternalHref(href) ? (
                  <Link to={href}>
                    {content}
                    {props.linkArrow === true && (
                      <ArrowUpRight size={15} aria-hidden />
                    )}
                  </Link>
                ) : (
                  content
                )}
                {actions.length > 0 && (
                  <div className={String(props.actionsClassName ?? "")}>
                    {actions.map((action) => (
                      <ActionControl
                        key={action.id}
                        action={action}
                        record={item}
                        dataMap={dataMap}
                        readOnly={readOnly}
                        emit={emit}
                      />
                    ))}
                  </div>
                )}
              </li>
            );
          })}
        </List>
      ) : (
        typeof props.emptyKey === "string" && (
          <p className="gov-empty">{t(props.emptyKey)}</p>
        )
      )}
    </>
  );
}
