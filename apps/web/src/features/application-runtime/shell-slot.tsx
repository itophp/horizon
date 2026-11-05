import { Suspense } from "react";
import { useApplicationCatalog } from "./catalog";
import { ContributionBoundary } from "./contributions";
import { extensions, LocalBoundary } from "./v2-registry";
import { Loading } from "@/features/administration/shared";
// The shell exposes fixed slots; deployed extensions cannot replace authentication or routing.
export function RuntimeShellSlot({
  slot,
}: {
  slot: "navigation" | "breadcrumb" | "toolbar" | "commands";
}) {
  const catalog = useApplicationCatalog();
  return (
    <>
      {(catalog.data?.items ?? [])
        .filter((app) =>
          app.extensions.some(
            (extension) =>
              extension.type === "shell" && extension.slot === slot,
          ),
        )
        .map((application) => (
          <LocalBoundary key={application.id}>
            <Suspense fallback={<Loading />}>
              <ContributionBoundary application={application}>
                {(active, readOnly) =>
                  readOnly ? null : (
                    <>
                      {active.extensions
                        .filter(
                          (extension) =>
                            extension.type === "shell" &&
                            extension.slot === slot,
                        )
                        .map((declaration) => {
                          const extension = extensions.resolve(
                            declaration.id,
                            active,
                          );
                          const View = extension.render;
                          if (!View)
                            throw new Error("component_not_registered");
                          extension.propsSchema?.parse({ record: {} });
                          return (
                            <LocalBoundary key={declaration.id}>
                              <View record={{}} />
                            </LocalBoundary>
                          );
                        })}
                    </>
                  )
                }
              </ContributionBoundary>
            </Suspense>
          </LocalBoundary>
        ))}
    </>
  );
}
