import { z } from "zod";
// Runtime records are projected data, not a second article DTO.
const scalar = z.union([z.string(), z.number(), z.boolean(), z.null()]);
export const recordSchema = z
  .object({ id: z.number().int().positive() })
  .catchall(scalar);
export type RuntimeRecord = z.infer<typeof recordSchema>;
export const listSchema = z.object({
  items: z.array(recordSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  pageSize: z.number().int().positive(),
});
