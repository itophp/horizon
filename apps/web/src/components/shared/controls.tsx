import { createContext, useContext, type ReactNode } from "react";
import { Checkbox } from "@/components/ui/checkbox";

// Generic controls shared by any feature. No business objects, field names or
// permission concepts appear here; callers own data, labels and localization.

export function initialsOf(name: string) {
  return name
    .split(/\s+/)
    .map((part) => part[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
}

export function EntityAvatar({
  name,
  seed = 0,
  size = "md",
}: {
  name: string;
  seed?: number;
  size?: "sm" | "md";
}) {
  const tones = 5;
  return (
    <span
      className={`person-avatar tone-${((seed % tones) + tones) % tones}`}
      aria-hidden="true"
      data-size={size}
    >
      {initialsOf(name)}
    </span>
  );
}

export type CascadeGroup = {
  id: string | number;
  label: string;
  children: Array<{ id: string | number; label: string }>;
};

const CascadeContext = createContext<{
  groups: CascadeGroup[];
  parentId: string | number;
  childId: string | number;
  choose: (parent: string | number, child: string | number) => void;
} | null>(null);

// Layout-preserving compound control: the provider renders no DOM, so callers
// can place the two selects in any grid while sharing cascade behavior.
export function CascadeSelect({
  groups,
  parentId,
  childId,
  onChange,
  children,
}: {
  groups: CascadeGroup[];
  parentId: string | number;
  childId: string | number;
  onChange: (parent: string | number, child: string | number) => void;
  children: ReactNode;
}) {
  return (
    <CascadeContext.Provider
      value={{ groups, parentId, childId, choose: onChange }}
    >
      {children}
    </CascadeContext.Provider>
  );
}

function useCascade() {
  const value = useContext(CascadeContext);
  if (!value) throw new Error("cascade_context_invalid");
  return value;
}

CascadeSelect.ParentSelect = function ParentSelect({
  placeholder,
  disabled,
}: {
  placeholder: string;
  disabled?: boolean;
}) {
  const { groups, parentId, choose } = useCascade();
  return (
    <select
      value={String(parentId)}
      disabled={disabled}
      onChange={(event) => choose(event.target.value, 0)}
    >
      <option value="0">{placeholder}</option>
      {groups.map((group) => (
        <option value={String(group.id)} key={String(group.id)}>
          {group.label}
        </option>
      ))}
    </select>
  );
};

CascadeSelect.ChildSelect = function ChildSelect({
  placeholder,
  disabled,
}: {
  placeholder: string;
  disabled?: boolean;
}) {
  const { groups, parentId, childId, choose } = useCascade();
  const group = groups.find((item) => String(item.id) === String(parentId));
  return (
    <select
      value={String(childId)}
      disabled={disabled}
      onChange={(event) => choose(parentId, event.target.value)}
    >
      <option value="0">{placeholder}</option>
      {(group?.children ?? []).map((child) => (
        <option value={String(child.id)} key={String(child.id)}>
          {child.label}
        </option>
      ))}
    </select>
  );
};

export function ChipGroup({
  items,
  onSelect,
}: {
  items: Array<{ key: string; label: string; selected?: boolean }>;
  onSelect: (key: string) => void;
}) {
  return (
    <div className="cell-tags">
      {items.map((item) => (
        <button
          type="button"
          key={item.key}
          className="neutral-badge"
          aria-pressed={item.selected}
          onClick={() => onSelect(item.key)}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}

export type CheckOption = {
  key: string;
  label: ReactNode;
  description?: ReactNode;
  badge?: ReactNode;
};

export function OptionCheckList({
  options,
  selected,
  onToggle,
  disabled,
}: {
  options: CheckOption[];
  selected: ReadonlyArray<string>;
  onToggle: (key: string, checked: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="option-check-list">
      {options.map((option) => (
        <label key={option.key}>
          <Checkbox
            disabled={disabled}
            checked={selected.includes(option.key)}
            onCheckedChange={(checked) =>
              onToggle(option.key, checked === true)
            }
          />
          <span className="role-option-content">
            <strong>{option.label}</strong>
            {option.description !== undefined && (
              <small>{option.description}</small>
            )}
          </span>
          {option.badge}
        </label>
      ))}
    </div>
  );
}
