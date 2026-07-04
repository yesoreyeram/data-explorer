import type { DataFrame } from "../../api/types";

export function chartableFields(frame: DataFrame) {
  const numeric = frame.schema.fields.filter((field) => field.type === "int64" || field.type === "float64").map((field) => field.name);
  const categorical = frame.schema.fields.filter((field) => field.type !== "json" && field.type !== "null").map((field) => field.name);
  return { numeric, categorical };
}
