import { useState } from "react";
import { Search } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { RuntimeFieldV2 } from "@/api/generated/runtime";
import type { Json } from "../expression";
import type { ComponentRendererProps } from "./component-registry";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxChips,
  ComboboxChipsInput,
  ComboboxChip,
  useComboboxAnchor,
} from "@/components/ui/combobox";
import { z } from "zod";

export type ImportOption = { value: string | number; label: string };
const optionResult = z.object({
  items: z.array(
    z.object({ value: z.union([z.string(), z.number()]), label: z.string() }),
  ),
});

export function ImportChoice({
  id,
  label,
  field,
  value,
  options,
  emit,
  update,
  remember,
  known,
}: {
  id: string;
  label: string;
  field: RuntimeFieldV2;
  value: Json | undefined;
  options: ImportOption[];
  emit: ComponentRendererProps["emit"];
  update: (value: Json) => void;
  remember: (items: ImportOption[]) => void;
  known: ImportOption[];
}) {
  const { t } = useTranslation();
  const anchor = useComboboxAnchor();
  const [remote, setRemote] = useState<ImportOption[]>([]);
  const [keyword, setKeyword] = useState("");
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState(false);
  const items = field.reference
    ? [
        ...remote,
        ...known.filter(
          (item) =>
            !remote.some((option) => option.value === item.value) &&
            (Array.isArray(value)
              ? value.includes(item.value)
              : value === item.value),
        ),
      ]
    : options;
  const selected = items.filter((item) =>
    Array.isArray(value) ? value.includes(item.value) : value === item.value,
  );
  const props = {
    items,
    itemToStringLabel: (item: ImportOption) => item.label,
    itemToStringValue: (item: ImportOption) => String(item.value),
    isItemEqualToValue: (item: ImportOption, other: ImportOption) =>
      item.value === other.value,
    ...(field.reference ? { filter: null } : {}),
  };
  async function search() {
    if (!keyword.trim() || busy) return;
    setBusy(true);
    setFailed(false);
    try {
      const ok = await emit(
        "search-" + field.name,
        { keyword: keyword.trim(), selected: value },
        {
          ephemeral: true,
          onResult: (data) => {
            const result = optionResult.safeParse(data);
            if (!result.success) {
              setFailed(true);
              return;
            }
            setRemote([
              ...selected.filter(
                (item) =>
                  !result.data.items.some(
                    (option) => option.value === item.value,
                  ),
              ),
              ...result.data.items,
            ]);
          },
          onError: () => setFailed(true),
        },
      );
      if (!ok) setFailed(true);
    } finally {
      setBusy(false);
    }
  }
  const list = (
    <ComboboxContent anchor={field.multiple ? anchor : undefined}>
      <ComboboxEmpty>{t("runtime:components.noOptions")}</ComboboxEmpty>
      <ComboboxList>
        {(item: ImportOption) => (
          <ComboboxItem key={item.value} value={item}>
            {item.label}
          </ComboboxItem>
        )}
      </ComboboxList>
    </ComboboxContent>
  );
  return (
    <>
      {field.reference && (
        <div className="runtime-import-search">
          <label className="sr-only" htmlFor={id + "-search"}>
            {label}
          </label>
          <Input
            id={id + "-search"}
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") void search();
            }}
          />
          <Button
            type="button"
            size="sm"
            onClick={search}
            disabled={busy || !keyword.trim()}
          >
            <Search aria-hidden />
            {t("runtime:batchImport.search")}
          </Button>
        </div>
      )}
      {failed && (
        <small role="alert">
          {t("runtime:batchImport.errors.unavailable")}
        </small>
      )}
      {field.multiple ? (
        <Combobox
          {...props}
          multiple
          value={selected}
          onValueChange={(next) => {
            if (field.reference) remember(next);
            update(next.map((item) => item.value));
          }}
        >
          <ComboboxChips ref={anchor}>
            {selected.map((item) => (
              <ComboboxChip key={item.value}>{item.label}</ComboboxChip>
            ))}
            <ComboboxChipsInput
              id={id}
              aria-label={label}
              autoFocus={!field.reference}
            />
          </ComboboxChips>
          {list}
        </Combobox>
      ) : (
        <Combobox
          {...props}
          value={selected[0] ?? null}
          onValueChange={(next) => {
            if (next) remember([next]);
            update(next?.value ?? null);
          }}
        >
          <ComboboxInput id={id} aria-label={label} />
          {list}
        </Combobox>
      )}
    </>
  );
}
