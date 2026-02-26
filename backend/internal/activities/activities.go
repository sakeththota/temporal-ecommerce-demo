package activities

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/db"
	"github.com/sakeththota/durable-embedding-migration/backend/internal/embeddings"
)

// Activities holds the dependencies needed by all activity functions.
type Activities struct {
	Pool   *pgxpool.Pool
	Ollama *embeddings.Client
}

// InitMigrationInput is the input for InitMigration.
type InitMigrationInput struct {
	Version    string
	ModelName  string
	Dimensions int
}

// InitMigrationResult is the output of InitMigration.
type InitMigrationResult struct {
	TotalRecords int
}

// InitMigration creates the version record and returns the total hotel count.
func (a *Activities) InitMigration(ctx context.Context, input InitMigrationInput) (InitMigrationResult, error) {
	time.Sleep(1 * time.Second)

	count, err := db.CountHotels(ctx, a.Pool)
	if err != nil {
		return InitMigrationResult{}, fmt.Errorf("counting hotels: %w", err)
	}

	if err := db.CreateVersion(ctx, a.Pool, input.Version, input.ModelName, input.Dimensions, count); err != nil {
		return InitMigrationResult{}, fmt.Errorf("creating version: %w", err)
	}

	slog.Info("initialized migration", "version", input.Version, "model", input.ModelName, "total", count)
	return InitMigrationResult{TotalRecords: count}, nil
}

// FetchBatchInput is the input for FetchBatch.
type FetchBatchInput struct {
	Offset int
	Limit  int
}

// HotelRecord is a minimal hotel representation passed between activities.
type HotelRecord struct {
	ID   string
	Text string // name + " " + description
}

// FetchBatchResult is the output of FetchBatch.
type FetchBatchResult struct {
	Hotels []HotelRecord
}

// FetchBatch retrieves a page of hotels for embedding.
func (a *Activities) FetchBatch(ctx context.Context, input FetchBatchInput) (FetchBatchResult, error) {
	time.Sleep(1 * time.Second)

	hotels, err := db.GetHotelBatch(ctx, a.Pool, input.Offset, input.Limit)
	if err != nil {
		return FetchBatchResult{}, fmt.Errorf("fetching batch: %w", err)
	}

	records := make([]HotelRecord, len(hotels))
	for i, h := range hotels {
		records[i] = HotelRecord{
			ID:   h.ID,
			Text: h.Name + " " + h.Description,
		}
	}

	return FetchBatchResult{Hotels: records}, nil
}

// GenerateAndStoreInput is the input for GenerateAndStoreEmbeddings.
type GenerateAndStoreInput struct {
	Hotels    []HotelRecord
	Version   string
	ModelName string
}

// GenerateAndStoreResult is the output of GenerateAndStoreEmbeddings.
type GenerateAndStoreResult struct {
	Processed int
}

// GenerateAndStoreEmbeddings embeds each hotel and saves to the database.
// Includes a 5% random failure rate to demonstrate Temporal retries.
func (a *Activities) GenerateAndStoreEmbeddings(ctx context.Context, input GenerateAndStoreInput) (GenerateAndStoreResult, error) {
	time.Sleep(2 * time.Second)

	// 5% failure injection for demo purposes.
	if rand.Float64() < 0.05 {
		return GenerateAndStoreResult{}, fmt.Errorf("simulated transient failure (5%% chance)")
	}

	for _, hotel := range input.Hotels {
		embedding, err := a.Ollama.Embed(ctx, input.ModelName, hotel.Text)
		if err != nil {
			return GenerateAndStoreResult{}, fmt.Errorf("embedding hotel %s: %w", hotel.ID, err)
		}

		if err := db.SaveEmbedding(ctx, a.Pool, hotel.ID, input.Version, input.ModelName, len(embedding), embedding); err != nil {
			return GenerateAndStoreResult{}, fmt.Errorf("saving embedding for hotel %s: %w", hotel.ID, err)
		}
	}

	return GenerateAndStoreResult{Processed: len(input.Hotels)}, nil
}

// UpdateProgressInput is the input for UpdateProgress.
type UpdateProgressInput struct {
	Version          string
	ProcessedRecords int
}

// UpdateProgress updates the processed count in the version record.
func (a *Activities) UpdateProgress(ctx context.Context, input UpdateProgressInput) error {
	time.Sleep(1 * time.Second)
	return db.UpdateVersionProgress(ctx, a.Pool, input.Version, input.ProcessedRecords)
}

// ValidateMigrationInput is the input for ValidateMigration.
type ValidateMigrationInput struct {
	Version      string
	TotalRecords int
}

// ValidateMigration checks that the right number of embeddings were created.
func (a *Activities) ValidateMigration(ctx context.Context, input ValidateMigrationInput) error {
	time.Sleep(2 * time.Second)

	embs, err := db.GetEmbeddingsByVersion(ctx, a.Pool, input.Version)
	if err != nil {
		return fmt.Errorf("getting embeddings for validation: %w", err)
	}

	if len(embs) != input.TotalRecords {
		return fmt.Errorf("validation failed: expected %d embeddings, got %d", input.TotalRecords, len(embs))
	}

	slog.Info("migration validated", "version", input.Version, "count", len(embs))
	return nil
}

// SwitchActiveVersionInput is the input for SwitchActiveVersion.
type SwitchActiveVersionInput struct {
	Version string
}

// SwitchActiveVersion marks the migration as completed and switches the active version.
func (a *Activities) SwitchActiveVersion(ctx context.Context, input SwitchActiveVersionInput) error {
	time.Sleep(1 * time.Second)

	if err := db.CompleteVersion(ctx, a.Pool, input.Version); err != nil {
		return fmt.Errorf("completing version: %w", err)
	}

	if err := db.SetActiveVersion(ctx, a.Pool, input.Version); err != nil {
		return fmt.Errorf("setting active version: %w", err)
	}

	slog.Info("switched active version", "version", input.Version)
	return nil
}

type BookingItemInput struct {
	HotelID       string
	CheckIn       time.Time
	CheckOut      time.Time
	Nights        int
	PricePerNight float64
	Subtotal      float64
}

type ValidateAvailabilityInput struct {
	WorkflowID string
	Items      []BookingItemInput
}

type ValidateAvailabilityResult struct {
	TotalAmount float64
}

func (a *Activities) ValidateAvailability(ctx context.Context, input ValidateAvailabilityInput) (ValidateAvailabilityResult, error) {
	slog.Info("validating availability", "workflowID", input.WorkflowID, "items", len(input.Items))

	time.Sleep(2 * time.Second)

	var total float64
	for _, item := range input.Items {
		total += item.Subtotal
	}

	if total <= 0 {
		return ValidateAvailabilityResult{}, fmt.Errorf("total amount must be greater than zero")
	}

	slog.Info("availability validated", "total", total)
	return ValidateAvailabilityResult{TotalAmount: total}, nil
}

type ProcessPaymentInput struct {
	WorkflowID string
	Amount     float64
	GuestEmail string
}

type ProcessPaymentResult struct {
	TransactionID string
}

func (a *Activities) ProcessPayment(ctx context.Context, input ProcessPaymentInput) (ProcessPaymentResult, error) {
	slog.Info("processing payment", "workflowID", input.WorkflowID, "amount", input.Amount)

	time.Sleep(3 * time.Second)

	if rand.Float64() < 0.2 {
		return ProcessPaymentResult{}, fmt.Errorf("simulated payment failure (20%% chance)")
	}

	transactionID := fmt.Sprintf("txn_%s", uuid.New().String()[:8])
	slog.Info("payment processed", "transactionID", transactionID)

	return ProcessPaymentResult{TransactionID: transactionID}, nil
}

type RefundPaymentInput struct {
	WorkflowID string
	Amount     float64
}

func (a *Activities) RefundPayment(ctx context.Context, input RefundPaymentInput) error {
	slog.Info("refunding payment", "workflowID", input.WorkflowID, "amount", input.Amount)
	time.Sleep(1 * time.Second)
	slog.Info("payment refunded", "workflowID", input.WorkflowID)
	return nil
}

type ReserveBookingInput struct {
	WorkflowID  string
	GuestName   string
	GuestEmail  string
	Items       []BookingItemInput
	TotalAmount float64
}

func (a *Activities) ReserveBooking(ctx context.Context, input ReserveBookingInput) error {
	slog.Info("reserving booking", "workflowID", input.WorkflowID, "guest", input.GuestName)

	time.Sleep(2 * time.Second)

	bookingID, err := db.CreateBooking(ctx, a.Pool, input.WorkflowID, input.GuestName, input.GuestEmail, input.TotalAmount)
	if err != nil {
		return fmt.Errorf("creating booking: %w", err)
	}

	for _, item := range input.Items {
		hotelID, err := uuid.Parse(item.HotelID)
		if err != nil {
			return fmt.Errorf("parsing hotel ID %s: %w", item.HotelID, err)
		}

		if err := db.AddBookingItem(ctx, a.Pool, bookingID, hotelID, item.CheckIn, item.CheckOut, item.Nights, item.PricePerNight); err != nil {
			return fmt.Errorf("adding booking item: %w", err)
		}
	}

	if err := db.UpdateBookingStatus(ctx, a.Pool, input.WorkflowID, "confirmed"); err != nil {
		return fmt.Errorf("updating booking status: %w", err)
	}

	slog.Info("booking reserved", "workflowID", input.WorkflowID, "bookingID", bookingID)
	return nil
}

type SendConfirmationInput struct {
	WorkflowID  string
	GuestName   string
	GuestEmail  string
	TotalAmount float64
}

func (a *Activities) SendConfirmation(ctx context.Context, input SendConfirmationInput) error {
	slog.Info("sending confirmation email", "workflowID", input.WorkflowID, "email", input.GuestEmail)
	time.Sleep(2 * time.Second)
	slog.Info("confirmation email sent", "workflowID", input.WorkflowID)
	return nil
}
