export interface Hotel {
  id: string;
  name: string;
  description: string;
  location: string;
  price_per_night: number;
  amenities: string[];
  created_at: string;
}

export interface SearchResult {
  hotel: Hotel;
  similarity: number;
}

export interface EmbeddingVersion {
  version: string;
  model_name: string;
  dimensions: number;
  status: string;
  total_records: number;
  processed_records: number;
  is_active: boolean;
  created_at: string;
  completed_at: string | null;
}

export interface MigrationProgress {
  status: string;
  total_records: number;
  processed_records: number;
  current_batch: number;
  started_at: string;
  last_activity_at: string;
}

export interface StartMigrationRequest {
  version: string;
  model_name: string;
  dimensions: number;
  batch_size: number;
}

export interface StartMigrationResponse {
  workflow_id: string;
  run_id: string;
  version: string;
}
