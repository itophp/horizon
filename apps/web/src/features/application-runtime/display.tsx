import { useTranslation } from "react-i18next";
import type { RuntimeFieldV2 } from "@/api/generated/runtime";
import {
  RegisteredDisplay,
  LocalBoundary,
  type RecordData,
} from "./v2-registry";
export function Fields({
  fields,
  record,
}: {
  fields: RuntimeFieldV2[];
  record: RecordData;
}) {
  const { t } = useTranslation();
  return (
    <dl className="runtime-fields">
      {fields
        .filter((f) => !f.hidden)
        .map((f) => {
          return (
            <div key={f.name}>
              <dt>{t(f.labelKey)}</dt>
              <dd>
                <LocalBoundary>
                  <RegisteredDisplay definition={f} value={record[f.name]} />
                </LocalBoundary>
              </dd>
            </div>
          );
        })}
    </dl>
  );
}
