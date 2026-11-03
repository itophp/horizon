import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export type EnumOption = { value: string; label: string };

/**
 * EnumSelect is the single place the console renders a closed value set. Radix
 * refuses an empty value, so callers pass a real sentinel ("all", "0") and map
 * it back when calling the capability.
 */
export function EnumSelect({
  id,
  value,
  onChange,
  options,
  disabled,
  placeholder,
}: {
  id?: string;
  value: string;
  onChange: (value: string) => void;
  options: EnumOption[];
  disabled?: boolean;
  placeholder?: string;
}) {
  return (
    <Select
      value={value}
      onValueChange={(next) => {
        if (next !== null) onChange(next);
      }}
      disabled={disabled}
    >
      <SelectTrigger id={id}>
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** targets lists the states reachable from the current one. */
export function targets(
  transitions: Record<string, string[]>,
  status: string,
): string[] {
  return transitions[status] ?? [];
}
