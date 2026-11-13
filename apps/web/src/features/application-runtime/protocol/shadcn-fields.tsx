import { useId } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Combobox,
  ComboboxContent,
  ComboboxChip,
  ComboboxChips,
  ComboboxChipsInput,
  useComboboxAnchor,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox";
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from "@/components/ui/input-otp";
import {
  NativeSelect,
  NativeSelectOption,
} from "@/components/ui/native-select";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Toggle } from "@/components/ui/toggle";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import type {
  ComponentRenderer,
  ComponentRendererProps,
} from "./component-registry";
import { useBoundValue, useUI } from "./shadcn-props";

function useControl(p: ComponentRendererProps) {
  const ui = useUI(p);
  const source =
    typeof ui.value === "string" ||
    typeof ui.value === "boolean" ||
    typeof ui.value === "number"
      ? ui.value
      : null;
  const [value, setValue] = useBoundValue<typeof source>(source);
  return {
    ...ui,
    value,
    update: setValue,
    attrs: { "aria-label": ui.label, disabled: ui.disabled },
    id: ui.string("id"),
    placeholder: ui.text(p.props.placeholderKey),
    initial: typeof value === "string" ? value : "",
    change: (next: typeof source) => {
      if (ui.disabled) return;
      setValue(next);
      ui.event("change", next);
    },
  };
}

function MultipleCombobox(p: ComponentRendererProps) {
  const { options, disabled, label, string, text, t, event } = useUI(p);
  const source = p.props.values ?? p.data;
  const [selected, setSelected] = useBoundValue(
    JSON.stringify(
      Array.isArray(source)
        ? source
            .slice(0, 500)
            .filter((value): value is string => typeof value === "string")
        : [],
    ),
  );
  const values = new Set<string>(JSON.parse(selected));
  const anchor = useComboboxAnchor();
  const selectedOptions = options.filter(({ value }) => values.has(value));
  return (
    <Combobox
      multiple
      items={options}
      disabled={disabled}
      value={selectedOptions}
      itemToStringLabel={(item) => item.label}
      itemToStringValue={(item) => item.value}
      isItemEqualToValue={(item, other) => item.value === other.value}
      onValueChange={(items) => {
        if (disabled) return;
        const next = [
          ...new Set([
            ...items.map(({ value }) => value),
            ...selectedOptions
              .filter((item) => item.disabled)
              .map(({ value }) => value),
          ]),
        ];
        setSelected(JSON.stringify(next));
        event("change", next);
      }}
    >
      <ComboboxChips ref={anchor}>
        {selectedOptions.map((item) => (
          <ComboboxChip
            key={item.value}
            aria-label={item.label}
            showRemove={!disabled && !item.disabled}
          >
            {item.label}
          </ComboboxChip>
        ))}
        <ComboboxChipsInput
          id={string("id")}
          aria-label={label}
          placeholder={text(p.props.placeholderKey)}
        />
      </ComboboxChips>
      <ComboboxContent anchor={anchor}>
        <ComboboxEmpty>{t("components.noOptions")}</ComboboxEmpty>
        <ComboboxList>
          {(item: (typeof options)[number]) => (
            <ComboboxItem
              key={item.value}
              value={item}
              disabled={item.disabled}
            >
              {item.label}
            </ComboboxItem>
          )}
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}

function useTextControl(p: ComponentRendererProps) {
  const control = useControl(p);
  const generatedId = useId();
  const id = control.id ?? generatedId;
  const error = control.text(p.props.errorKey);
  const invalid = p.props.invalid === true;
  const minLength = control.number("minLength");
  const maxLength = control.number("maxLength");
  const type = p.props.type;
  return {
    ...control,
    id,
    error: invalid ? error : undefined,
    type:
      type === "email" ||
      type === "url" ||
      type === "tel" ||
      type === "search" ||
      type === "number"
        ? type
        : "text",
    attrs: {
      ...control.attrs,
      "aria-invalid": invalid || undefined,
      "aria-describedby":
        [
          control.string("describedBy"),
          invalid && error ? id + "-error" : undefined,
        ]
          .filter(Boolean)
          .join(" ") || undefined,
      required: p.props.required === true,
      minLength:
        minLength === undefined
          ? undefined
          : Math.max(0, Math.trunc(minLength)),
      maxLength:
        maxLength === undefined
          ? undefined
          : Math.max(0, Math.trunc(maxLength)),
    },
  };
}

export const fieldRenderers = {
  checkbox: function Check(p) {
    const { attrs, id, value, change } = useControl(p);
    return (
      <Checkbox
        {...attrs}
        id={id}
        checked={value === true}
        onCheckedChange={change}
      />
    );
  },
  switch: function SwitchControl(p) {
    const { attrs, id, value, change } = useControl(p);
    return (
      <Switch
        {...attrs}
        id={id}
        checked={value === true}
        onCheckedChange={change}
      />
    );
  },
  input: function TextInput(p) {
    const { attrs, id, placeholder, initial, change, type, error } =
      useTextControl(p);
    return (
      <>
        <Input
          {...attrs}
          id={id}
          type={type}
          placeholder={placeholder}
          value={initial}
          onChange={(e) => change(e.currentTarget.value)}
        />
        {error && <FieldError id={id + "-error"}>{error}</FieldError>}
      </>
    );
  },
  textarea: function TextArea(p) {
    const { attrs, id, placeholder, initial, change, error, number } =
      useTextControl(p);
    return (
      <>
        <Textarea
          {...attrs}
          id={id}
          rows={number("rows")}
          placeholder={placeholder}
          value={initial}
          onChange={(e) => change(e.currentTarget.value)}
        />
        {error && <FieldError id={id + "-error"}>{error}</FieldError>}
      </>
    );
  },
  field: function FieldGroup(p) {
    const { label, description, string, text } = useUI(p);
    const target = string("for");
    const invalid = p.props.invalid === true;
    const error = invalid ? text(p.props.errorKey) : undefined;
    return (
      <Field
        data-invalid={invalid || undefined}
        orientation={
          p.props.orientation === "horizontal" ||
          p.props.orientation === "responsive"
            ? p.props.orientation
            : "vertical"
        }
      >
        <FieldLabel htmlFor={target}>{label}</FieldLabel>
        {p.children}
        {description && (
          <FieldDescription id={target ? target + "-description" : undefined}>
            {description}
          </FieldDescription>
        )}
        {error && (
          <FieldError id={target ? target + "-field-error" : undefined}>
            {error}
          </FieldError>
        )}
      </Field>
    );
  },
  "input-group": function GroupedInput(p) {
    const {
      attrs,
      id,
      placeholder,
      initial,
      description,
      change,
      type,
      error,
    } = useTextControl(p);
    return (
      <>
        <InputGroup>
          <InputGroupInput
            {...attrs}
            id={id}
            type={type}
            placeholder={placeholder}
            value={initial}
            onChange={(e) => change(e.currentTarget.value)}
          />
          {description && <InputGroupAddon>{description}</InputGroupAddon>}
        </InputGroup>
        {error && <FieldError id={id + "-error"}>{error}</FieldError>}
      </>
    );
  },
  "input-otp": function OtpInput(p) {
    const { attrs, id, number } = useControl(p);
    const length = Math.max(1, Math.min(12, Math.trunc(number("length") ?? 6)));
    return (
      <InputOTP
        {...attrs}
        id={id}
        maxLength={length}
        inputMode="numeric"
        autoComplete="one-time-code"
        pattern="[0-9]*"
        onComplete={(value) => {
          if (!attrs.disabled)
            void p.emit("complete", value, { ephemeral: true });
        }}
      >
        <InputOTPGroup>
          {Array.from({ length }, (_, index) => (
            <InputOTPSlot key={index} index={index} />
          ))}
        </InputOTPGroup>
      </InputOTP>
    );
  },
  "native-select": function NativeChoice(p) {
    const { attrs, id, initial, placeholder, options, change } = useControl(p);
    return (
      <NativeSelect
        {...attrs}
        id={id}
        value={initial}
        onChange={(e) => change(e.currentTarget.value)}
      >
        <NativeSelectOption value="" disabled>
          {placeholder}
        </NativeSelectOption>
        {options.map(({ value, label, disabled }) => (
          <NativeSelectOption key={value} value={value} disabled={disabled}>
            {label}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    );
  },
  select: function SelectChoice(p) {
    const { attrs, id, initial, placeholder, options, change } = useControl(p);
    return (
      <Select
        disabled={attrs.disabled}
        value={initial || null}
        items={options}
        onValueChange={(next) => change(typeof next === "string" ? next : null)}
      >
        <SelectTrigger id={id} aria-label={attrs["aria-label"]}>
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          {options.map(({ value, label, disabled }) => (
            <SelectItem key={value} value={value} disabled={disabled}>
              {label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  },
  "radio-group": function RadioChoice(p) {
    const { attrs, initial, options, change } = useControl(p);
    const id = useId();
    return (
      <RadioGroup
        {...attrs}
        value={initial}
        onValueChange={(next) => change(typeof next === "string" ? next : null)}
      >
        {options.map(({ value, label, disabled }, index) => (
          <div key={value} className="flex items-center gap-2">
            <RadioGroupItem id={id + index} value={value} disabled={disabled} />
            <FieldLabel htmlFor={id + index}>{label}</FieldLabel>
          </div>
        ))}
      </RadioGroup>
    );
  },
  combobox: function FilteredChoice(p) {
    const { attrs, id, initial, options, placeholder, t, change } =
      useControl(p);
    if (p.props.multiple === true) return <MultipleCombobox {...p} />;
    return (
      <Combobox
        items={options}
        disabled={attrs.disabled}
        value={options.find(({ value }) => value === initial) ?? null}
        itemToStringLabel={(item) => item.label}
        itemToStringValue={(item) => item.value}
        isItemEqualToValue={(item, value) => item.value === value.value}
        onValueChange={(item) => change(item?.value ?? null)}
      >
        <ComboboxInput
          id={id}
          aria-label={attrs["aria-label"]}
          placeholder={placeholder}
          showTrigger={false}
        />
        <ComboboxContent>
          <ComboboxEmpty>{t("components.noOptions")}</ComboboxEmpty>
          <ComboboxList>
            {(item: (typeof options)[number]) => (
              <ComboboxItem
                key={item.value}
                value={item}
                disabled={item.disabled}
              >
                {item.label}
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    );
  },
  slider: function Range(p) {
    const { attrs, id, value, number, change, update, label, t } =
      useControl(p);
    const min = number("min") ?? 0;
    const configuredMax = number("max") ?? 100;
    const max = configuredMax > min ? configuredMax : min + 1;
    const clamp = (value: number) => Math.max(min, Math.min(max, value));
    const initial =
      typeof value === "number" && Number.isFinite(value) ? clamp(value) : min;
    const rangeSource = p.props.range ?? p.data;
    const endpoints = Array.isArray(rangeSource)
      ? rangeSource
          .filter(
            (value): value is number =>
              typeof value === "number" && Number.isFinite(value),
          )
          .slice(0, 2)
      : [];
    const [range, setRange] = useBoundValue(
      JSON.stringify(
        [clamp(endpoints[0] ?? min), clamp(endpoints[1] ?? max)].sort(
          (a, b) => a - b,
        ),
      ),
    );
    const isRange = p.props.mode === "range";
    return (
      <Slider
        disabled={attrs.disabled}
        id={id}
        min={min}
        max={max}
        orientation={
          p.props.orientation === "vertical" ? "vertical" : "horizontal"
        }
        step={Math.max(0.000001, number("step") ?? 1)}
        value={isRange ? (JSON.parse(range) as number[]) : [initial]}
        getAriaLabel={(index) =>
          isRange
            ? t(index === 0 ? "components.rangeStart" : "components.rangeEnd", {
                label,
              })
            : (label ?? "")
        }
        onValueChange={(next) => {
          if (isRange) setRange(JSON.stringify(next));
          else update(Array.isArray(next) ? next[0] : next);
        }}
        onValueCommitted={(next) => {
          if (isRange) {
            if (!attrs.disabled) void p.emit("change", next);
          } else change(Array.isArray(next) ? next[0] : next);
        }}
      />
    );
  },
  toggle: function ToggleControl(p) {
    const { attrs, value, label, change } = useControl(p);
    return (
      <Toggle {...attrs} pressed={value === true} onPressedChange={change}>
        {label}
      </Toggle>
    );
  },
  "toggle-group": function Toggles(p) {
    const { attrs, initial, options, change, event } = useControl(p);
    const multiple = p.props.multiple === true;
    const source = p.props.values ?? p.data;
    const [selected, setSelected] = useBoundValue(
      JSON.stringify(
        Array.isArray(source)
          ? source
              .filter((value): value is string => typeof value === "string")
              .slice(0, 500)
          : [],
      ),
    );
    return (
      <ToggleGroup
        {...attrs}
        multiple={multiple}
        value={
          multiple
            ? (JSON.parse(selected) as string[])
            : initial
              ? [initial]
              : []
        }
        onValueChange={(value: string[]) => {
          if (attrs.disabled) return;
          if (multiple) {
            setSelected(JSON.stringify(value));
            event("change", value);
          } else change(value[0] ?? null);
        }}
      >
        {options.map(({ value, label, disabled }) => (
          <ToggleGroupItem key={value} value={value} disabled={disabled}>
            {label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    );
  },
} satisfies Record<string, ComponentRenderer>;
