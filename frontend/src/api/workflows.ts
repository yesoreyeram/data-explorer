import { api } from "./client";
import type { DataFrame, Workflow, WorkflowDefinition, WorkflowExecution, WorkflowStatus } from "./types";

export interface WorkflowInput {
  name: string;
  description: string;
  definition: WorkflowDefinition;
  status?: WorkflowStatus;
}

export async function listWorkflows(): Promise<Workflow[]> {
  const res = await api.get<Workflow[]>("/workflows/");
  return res.data ?? [];
}

export async function getWorkflow(id: string): Promise<Workflow> {
  const res = await api.get<Workflow>(`/workflows/${id}`);
  return res.data;
}

export async function createWorkflow(input: WorkflowInput): Promise<Workflow> {
  const res = await api.post<Workflow>("/workflows/", input);
  return res.data;
}

export async function updateWorkflow(id: string, input: WorkflowInput): Promise<Workflow> {
  const res = await api.put<Workflow>(`/workflows/${id}`, input);
  return res.data;
}

export async function deleteWorkflow(id: string): Promise<void> {
  await api.delete(`/workflows/${id}`);
}

export interface ExecuteWorkflowResponse {
  execution: WorkflowExecution;
  output?: DataFrame;
  error?: string;
}

export async function executeWorkflow(id: string): Promise<ExecuteWorkflowResponse> {
  const res = await api.post<ExecuteWorkflowResponse>(`/workflows/${id}/execute`);
  return res.data;
}

export async function listWorkflowExecutions(id: string, limit = 50): Promise<WorkflowExecution[]> {
  const res = await api.get<WorkflowExecution[]>(`/workflows/${id}/executions`, { params: { limit } });
  return res.data ?? [];
}
