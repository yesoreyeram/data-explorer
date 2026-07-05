import { api } from "./client";
import type { GuardrailStatsResponse } from "./types";

export async function getGuardrailStats() {
  const res = await api.get<GuardrailStatsResponse>("/admin/guardrails/stats");
  return res.data;
}
