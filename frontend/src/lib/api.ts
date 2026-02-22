import type {
  SearchResult,
  EmbeddingVersion,
  MigrationProgress,
  StartMigrationRequest,
  StartMigrationResponse,
} from "./types";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });

  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API error ${res.status}: ${body}`);
  }

  return res.json() as Promise<T>;
}

export async function searchHotels(
  query: string,
  limit = 10
): Promise<SearchResult[]> {
  const params = new URLSearchParams({ q: query, limit: String(limit) });
  return fetchJSON<SearchResult[]>(`/api/search?${params}`);
}

export async function getVersions(): Promise<EmbeddingVersion[]> {
  return fetchJSON<EmbeddingVersion[]>("/api/versions");
}

export async function startMigration(
  req: StartMigrationRequest
): Promise<StartMigrationResponse> {
  return fetchJSON<StartMigrationResponse>("/api/migrations", {
    method: "POST",
    body: JSON.stringify(req),
  });
}

export async function getMigrationProgress(
  version: string
): Promise<MigrationProgress> {
  return fetchJSON<MigrationProgress>(
    `/api/migrations/${encodeURIComponent(version)}`
  );
}

export async function pauseMigration(
  version: string
): Promise<{ status: string; version: string }> {
  return fetchJSON(`/api/migrations/${encodeURIComponent(version)}/pause`, {
    method: "POST",
  });
}

export async function resumeMigration(
  version: string
): Promise<{ status: string; version: string }> {
  return fetchJSON(`/api/migrations/${encodeURIComponent(version)}/resume`, {
    method: "POST",
  });
}
