import type {
  SearchResult,
  Hotel,
  EmbeddingVersion,
  MigrationProgress,
  ApprovalMigrationProgress,
  StartMigrationRequest,
  StartMigrationResponse,
  CreateBookingRequest,
  BookingProgress,
  Booking,
} from "./types";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL ?? "";
const BOOKING_API_BASE =
  process.env.NEXT_PUBLIC_BOOKING_API_URL ?? "";

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

async function fetchBookingJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BOOKING_API_BASE}${path}`, {
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
): Promise<MigrationProgress | ApprovalMigrationProgress> {
  return fetchJSON<MigrationProgress | ApprovalMigrationProgress>(
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

export async function updateMigration(
  version: string,
  batchSize: number
): Promise<{ status: string; version: string }> {
  return fetchJSON(`/api/migrations/${encodeURIComponent(version)}/update`, {
    method: "POST",
    body: JSON.stringify({ batch_size: batchSize }),
  });
}

export async function approveMigration(
  version: string
): Promise<{ status: string; version: string }> {
  return fetchJSON(`/api/migrations/${encodeURIComponent(version)}/approve`, {
    method: "POST",
  });
}

export async function rejectMigration(
  version: string
): Promise<{ status: string; version: string }> {
  return fetchJSON(`/api/migrations/${encodeURIComponent(version)}/reject`, {
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
  return fetchBookingJSON<{ workflow_id: string }>("/api/bookings", {
    method: "POST",
    body: JSON.stringify(req),
  });
}

export async function getBookingProgress(
  workflowId: string
): Promise<BookingProgress> {
  return fetchBookingJSON<BookingProgress>(
    `/api/bookings/${encodeURIComponent(workflowId)}`
  );
}

export async function cancelBooking(
  workflowId: string
): Promise<{ status: string; workflow_id: string }> {
  return fetchBookingJSON(`/api/bookings/${encodeURIComponent(workflowId)}/cancel`, {
    method: "POST",
  });
}

export async function listBookings(): Promise<Booking[]> {
  return fetchBookingJSON<Booking[]>("/api/bookings");
}

export async function crashServer(): Promise<{ status: string }> {
  return fetchBookingJSON<{ status: string }>("/api/bookings/crash", {
    method: "POST",
  });
}

export async function crashSearchServer(): Promise<{ status: string }> {
  return fetchJSON<{ status: string }>("/api/crash", {
    method: "POST",
  });
}

export async function checkBookingHealth(): Promise<boolean> {
  try {
    const res = await fetch(`${BOOKING_API_BASE}/api/bookings/health`, {
      cache: "no-store",
      headers: { "Cache-Control": "no-cache" },
    });
    return res.ok;
  } catch {
    return false;
  }
}

export async function checkSearchHealth(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/api/health`, {
      cache: "no-store",
      headers: { "Cache-Control": "no-cache" },
    });
    return res.ok;
  } catch {
    return false;
  }
}
