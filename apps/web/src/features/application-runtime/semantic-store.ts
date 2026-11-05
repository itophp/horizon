import type { AIPage } from "@/api/generated/models";
// Only safe page references are handed to the assistant; no form values or record bodies.
let current: { owner: string; page: AIPage } | undefined;
export function publishPageReference(owner: string, page: AIPage) {
  current = { owner, page };
  return () => {
    if (current?.owner === owner) current = undefined;
  };
}
export function getPageReference() {
  return current?.page;
}
