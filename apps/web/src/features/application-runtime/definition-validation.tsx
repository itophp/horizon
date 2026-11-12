import { Component, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { runtimeApplicationV2Schema } from "@/api/generated/runtime";

export function validateDefinition(raw: unknown) {
  const application = runtimeApplicationV2Schema.parse(raw);
  if (
    !application.protocol ||
    application.schemaVersion !== "3.0" ||
    application.runtimeCompatibility !== "3.0" ||
    application.protocol.schemaVersion !== "3.0" ||
    application.protocol.runtimeCompatibility !== "3.0" ||
    application.protocol.pages.some((page) => page.schemaVersion !== "3.0")
  )
    throw new Error("schema_version_unsupported");
  return application;
}

const diagnosticCodes = new Set([
  "schema_invalid",
  "schema_version_unsupported",
  "component_not_registered",
  "action_not_found",
  "field_not_allowed",
  "page_not_found",
  "runtime_context_invalid",
]);

function diagnosticCode(error: unknown) {
  const code = error instanceof Error ? error.message.split(":")[0] : "";
  return diagnosticCodes.has(code) ? code : "schema_invalid";
}

export function Diagnostic({ error }: { error: unknown }) {
  const { t } = useTranslation("runtime");
  return (
    <div role="alert">
      {t("schemaError")} <code>{diagnosticCode(error)}</code>
    </div>
  );
}

export class LocalBoundary extends Component<
  { children: ReactNode },
  { failed: boolean; error: unknown }
> {
  state = { failed: false, error: undefined };
  static getDerivedStateFromError(error: unknown) {
    return { failed: true, error };
  }
  render() {
    return this.state.failed ? (
      <Diagnostic error={this.state.error} />
    ) : (
      this.props.children
    );
  }
}
