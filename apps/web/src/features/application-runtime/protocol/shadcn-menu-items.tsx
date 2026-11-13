import {
  ContextMenuCheckboxItem,
  ContextMenuItem,
  ContextMenuRadioGroup,
  ContextMenuRadioItem,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
} from "@/components/ui/context-menu";
import {
  DropdownMenuCheckboxItem,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/components/ui/dropdown-menu";
import { useBoundValue } from "./shadcn-props";

type MenuItem = {
  value: string;
  labelKey: string;
  disabled?: boolean;
  kind?: string;
  checked?: boolean;
  selected?: string;
  options?: unknown;
};

function isMenuItem(value: unknown): value is MenuItem {
  return (
    !!value &&
    typeof value === "object" &&
    "value" in value &&
    typeof value.value === "string" &&
    "labelKey" in value &&
    typeof value.labelKey === "string"
  );
}

const primitives = {
  dropdown: {
    Item: DropdownMenuItem,
    Check: DropdownMenuCheckboxItem,
    Radio: DropdownMenuRadioItem,
    RadioGroup: DropdownMenuRadioGroup,
    Sub: DropdownMenuSub,
    Trigger: DropdownMenuSubTrigger,
    Content: DropdownMenuSubContent,
  },
  context: {
    Item: ContextMenuItem,
    Check: ContextMenuCheckboxItem,
    Radio: ContextMenuRadioItem,
    RadioGroup: ContextMenuRadioGroup,
    Sub: ContextMenuSub,
    Trigger: ContextMenuSubTrigger,
    Content: ContextMenuSubContent,
  },
};

type MenuSelection =
  | string
  | { value: string; checked: boolean }
  | { group: string; value: string };

type ItemsProps = {
  items: unknown;
  variant: keyof typeof primitives;
  disabled: boolean;
  t: (key: string) => string;
  select: (selection: MenuSelection) => void;
  depth?: number;
};

export function RuntimeMenuItems({ items, depth = 0, ...props }: ItemsProps) {
  if (!Array.isArray(items) || depth > 2) return null;
  const seen = new Set<string>();
  return items
    .slice(0, depth ? 20 : 50)
    .filter(isMenuItem)
    .map((item) => {
      if (seen.has(item.value)) return null;
      seen.add(item.value);
      return (
        <RuntimeMenuItem
          key={item.value}
          {...props}
          depth={depth}
          item={item}
        />
      );
    });
}

function RuntimeMenuItem({
  item,
  variant,
  disabled,
  t,
  select,
  depth = 0,
}: Omit<ItemsProps, "items"> & { item: MenuItem }) {
  const { Item, Check, Radio, RadioGroup, Sub, Trigger, Content } =
    primitives[variant];
  const [checked, setChecked] = useBoundValue(item.checked === true);
  const [selected, setSelected] = useBoundValue(
    typeof item.selected === "string" ? item.selected : "",
  );
  const locked = disabled || item.disabled === true;
  const label = t(item.labelKey);
  switch (item.kind) {
    case "checkbox":
      return (
        <Check
          disabled={locked}
          checked={checked}
          onCheckedChange={(next) => {
            if (locked) return;
            setChecked(next);
            select({ value: item.value, checked: next });
          }}
        >
          {label}
        </Check>
      );
    case "radio":
      return (
        <RadioGroup
          aria-label={label}
          value={selected}
          onValueChange={(next) => {
            if (locked || typeof next !== "string") return;
            setSelected(next);
            select({ group: item.value, value: next });
          }}
        >
          {Array.isArray(item.options) &&
            item.options
              .slice(0, 20)
              .filter(isMenuItem)
              .map((option) => (
                <Radio
                  key={option.value}
                  value={option.value}
                  disabled={locked || option.disabled === true}
                >
                  {t(option.labelKey)}
                </Radio>
              ))}
        </RadioGroup>
      );
    case "submenu":
      return (
        depth < 2 && (
          <Sub>
            <Trigger disabled={locked}>{label}</Trigger>
            <Content>
              <RuntimeMenuItems
                items={item.options}
                variant={variant}
                disabled={locked}
                t={t}
                select={select}
                depth={depth + 1}
              />
            </Content>
          </Sub>
        )
      );
    default:
      return (
        <Item disabled={locked} onClick={() => select(item.value)}>
          {label}
        </Item>
      );
  }
}
