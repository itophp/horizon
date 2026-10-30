import { defineConfig, defineTransformer } from "orval";
export default defineConfig({
  baseline: {
    input: {
      target: "../../services/api/gen/http/openapi3.json",
      // Goa JSON bytes are base64 strings. Orval otherwise models format:binary as Blob.
      override: {
        transformer: defineTransformer((spec) => {
          const walk = (node: unknown): void => {
            if (Array.isArray(node)) {
              for (const item of node) walk(item);
              return;
            }
            if (typeof node !== "object" || node === null) return;
            const object = node as Record<string, unknown>;
            if (object.type === "string" && object.format === "binary")
              delete object.format;
            for (const value of Object.values(object)) walk(value);
          };
          walk(spec);
          return spec;
        }),
      },
    },
    output: {
      target: "src/api/generated/baseline.ts",
      schemas: "src/api/generated/models",
      client: "react-query",
      httpClient: "fetch",
      clean: true,
      override: {
        mutator: { path: "./src/api/client/request.ts", name: "request" },
      },
    },
  },
});
