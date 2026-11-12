import { readFileSync } from "node:fs";
import { MemoryRouter } from "react-router";
import { ApplicationPage } from "@/features/application-runtime/page";
import type { RuntimeApplicationV2 } from "@/api/generated/runtime";

const applications = JSON.parse(
  readFileSync("../../docs/generated/runtime/applications.json", "utf8"),
) as RuntimeApplicationV2[];
export const capabilityDefinition = applications.find(
  (application) => application.id === "capabilities",
)!;

export function CapabilityRuntimeFixture() {
  return (
    <MemoryRouter initialEntries={["/capabilities"]}>
      <ApplicationPage
        applicationId="capabilities"
        pageId="capabilities.console"
      />
    </MemoryRouter>
  );
}
