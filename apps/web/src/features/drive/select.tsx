import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
export function DriveSelect<T extends string>({
  id,
  label,
  value,
  options,
  disabled,
  onValueChange,
  onBlur,
}: {
  id?: string;
  label: string;
  value: T;
  options: readonly { value: T; label: string }[];
  disabled?: boolean;
  onValueChange: (value: T) => void;
  onBlur?: () => void;
}) {
  return (
    <Select
      value={value}
      disabled={disabled}
      onValueChange={(value) => {
        if (value !== null) onValueChange(value);
      }}
    >
      <SelectTrigger id={id} aria-label={label} onBlur={onBlur}>
        <SelectValue>
          {options.find((option) => option.value === value)?.label}
        </SelectValue>
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
