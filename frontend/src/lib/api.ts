import type {
  SearchResult,
  Hotel,
  EmbeddingVersion,
  MigrationProgress,
  StartMigrationRequest,
  StartMigrationResponse,
  CreateBookingRequest,
  BookingProgress,
  Booking,
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

export async function getHotels(): Promise<Hotel[]> {
  return fetchJSON<Hotel[]>("/api/hotels");
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

export async function resetMigrations(): Promise<{ status: string }> {
  return fetchJSON(`/api/migrations/reset`, {
    method: "POST",
  });
}

export async function createBooking(
  req: CreateBookingRequest
): Promise<{ workflow_id: string }> {
  return fetchJSON<{ workflow_id: string }>("/api/bookings", {
    method: "POST",
    body: JSON.stringify(req),
  });
}

export async function getBookingProgress(
  workflowId: string
): Promise<BookingProgress> {
  return fetchJSON<BookingProgress>(
    `/api/bookings/${encodeURIComponent(workflowId)}`
  );
}

export async function listBookings(): Promise<Booking[]> {
  return fetchJSON<Booking[]>("/api/bookings");
}

export async function crashServer(): Promise<{ status: string }> {
  return fetchJSON<{ status: string }>("/api/crash", {
    method: "POST",
  });
}
