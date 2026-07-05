# src/api

## What this package does

`src/api` contains **typed, resource-scoped fetch wrappers** for every backend API endpoint. Each file owns one resource area and exports typed async functions that the rest of the application calls. The central `client.ts` handles auth header injection, silent access-token refresh, and base URL configuration.

## Files

| File | Resource | Key exports |
|---|---|---|
| `client.ts` | HTTP client | `apiClient` (axios instance with interceptors) |
| `auth.ts` | Authentication | `login`, `register`, `logout`, `refresh`, `changePassword` |
| `connections.ts` | Connections | `listConnections`, `getConnection`, `createConnection`, `updateConnection`, `deleteConnection`, `testConnection`, `queryConnection` |
| `explore.ts` | Ad-hoc exploration | `runExploreQuery` (saved connection or inline temp connection) |
| `workflows.ts` | Workflows | `listWorkflows`, `getWorkflow`, `createWorkflow`, `updateWorkflow`, `deleteWorkflow`, `executeWorkflow`, `getExecutions`, `setSchedule` |
| `catalog.ts` | Integration catalog | `listCatalogEntries`, `getCatalogEntry`, `searchCatalog` |
| `audit.ts` | Audit log | `listAuditLogs` |
| `users.ts` | User management | `listUsers`, `updateUser`, `assignRole`, `removeRole` |
| `search.ts` | Global search | `globalSearch` |
| `admin.ts` | Admin utilities | `listRoles`, `getGuardrailStatus` |
| `types.ts` | Shared types | `DataFrame`, `Connection`, `Workflow`, `AuditLog`, `User`, etc. |

## client.ts: central HTTP configuration

```typescript
const apiClient = axios.create({
    baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
});

// Request interceptor: inject Authorization: ******
apiClient.interceptors.request.use((config) => {
    const token = authStore.getState().accessToken;
    if (token) config.headers.Authorization = `******;
    return config;
});

// Response interceptor: on 401, attempt silent token refresh, retry once
apiClient.interceptors.response.use(…, async (error) => {
    if (error.response?.status === 401) {
        await refresh();  // rotates refresh token cookie, gets new access token
        return apiClient(error.config);  // retry original request
    }
    return Promise.reject(error);
});
```

## types.ts: the DataFrame wire type

```typescript
export interface DataFrame {
    schema: Array<{ name: string; type: string; nullable: boolean }>;
    rows: any[][];
    meta: {
        sourceType?: string;
        sourceId?: string;
        rowCount: number;
        truncated: boolean;
        warnings?: string[];
        // …
    };
}
```

This matches `pkg/dataframe`'s JSON wire format and is used by `DataFrameView`, `DataTable`, `ExplorePage`, and workflow execution results.

## Architecture decisions (ADRs)

| Decision | Rationale |
|---|---|
| One file per resource | Mirrors the backend handler layout; easy to find the function for any endpoint |
| Silent refresh in `client.ts` interceptor | Transparent to the rest of the app; callers never see a 401 if the refresh token is valid |
| TypeScript types in `types.ts` | Single source of truth for API shapes; avoids drift between callers |
| Axios (not fetch) | Interceptors, automatic JSON parsing, and consistent error shapes |

## Scope and responsibilities

- Wrap every API endpoint in a typed async function.
- Handle authentication headers and silent token refresh.
- Export TypeScript types for all API request/response shapes.

## Limitations and todos

- [ ] No OpenAPI-generated client; types in `types.ts` are hand-maintained and can drift from the backend.
- [ ] Error handling is not consistently typed; API errors are currently `any`.
- [ ] No request deduplication (calling the same endpoint twice in parallel makes two requests).
- [ ] Refresh retry is attempted only once; a second 401 after refresh propagates to the caller.
