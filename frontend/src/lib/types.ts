export interface Hotel {
  id: string;
  name: string;
  description: string;
  location: string;
  price_per_night: number;
  amenities: string[];
  image_url: string;
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

export interface BookingItem {
  hotel_id: string;
  check_in: string;
  check_out: string;
  nights: number;
  price_per_night: number;
  subtotal: number;
}

export interface CreateBookingRequest {
  guest_name: string;
  guest_email: string;
  items: BookingItem[];
}

export interface BookingProgress {
  status: string;
  guest_name: string;
  guest_email: string;
  total_amount: number;
  current_step: string;
  error?: string;
  started_at: string;
  completed_at?: string;
}

export interface Booking {
  id: string;
  workflow_id: string;
  guest_name: string;
  guest_email: string;
  total_amount: number;
  status: string;
  created_at: string;
  updated_at: string;
}
