package activities

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"

	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/db"
	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/embeddings"
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
	activity.RecordHeartbeat(ctx, "initializing")
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
	activity.RecordHeartbeat(ctx, "fetching")
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
	activity.RecordHeartbeat(ctx, "starting")
	time.Sleep(2 * time.Second)

	// 5% failure injection for demo purposes.
	if rand.Float64() < 0.05 {
		return GenerateAndStoreResult{}, fmt.Errorf("simulated transient failure (5%% chance)")
	}

	for i, hotel := range input.Hotels {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("embedding %d/%d", i+1, len(input.Hotels)))
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
	activity.RecordHeartbeat(ctx, "updating")
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
	activity.RecordHeartbeat(ctx, "validating")
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
	activity.RecordHeartbeat(ctx, "switching")
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

// BookingItemInput represents a single hotel booking line item.
type BookingItemInput struct {
	HotelID       string
	CheckIn       time.Time
	CheckOut      time.Time
	Nights        int
	PricePerNight float64
	Subtotal      float64
}

// ValidateAvailabilityInput is the input for ValidateAvailability.
type ValidateAvailabilityInput struct {
	WorkflowID string
	Items      []BookingItemInput
}

// ValidateAvailabilityResult is the output of ValidateAvailability.
type ValidateAvailabilityResult struct {
	TotalAmount float64
}

// ValidateAvailability checks that all requested items are available and computes the total.
func (a *Activities) ValidateAvailability(ctx context.Context, input ValidateAvailabilityInput) (ValidateAvailabilityResult, error) {
	activity.RecordHeartbeat(ctx, "validating availability")
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

// ProcessPaymentInput is the input for ProcessPayment.
type ProcessPaymentInput struct {
	WorkflowID string
	Amount     float64
	GuestEmail string
}

// ProcessPaymentResult is the output of ProcessPayment.
type ProcessPaymentResult struct {
	TransactionID string
}

// ProcessPayment simulates charging the guest. Includes a 20% failure rate for demo purposes.
func (a *Activities) ProcessPayment(ctx context.Context, input ProcessPaymentInput) (ProcessPaymentResult, error) {
	activity.RecordHeartbeat(ctx, "processing payment")
	slog.Info("processing payment", "workflowID", input.WorkflowID, "amount", input.Amount)

	time.Sleep(3 * time.Second)

	if rand.Float64() < 0.2 {
		return ProcessPaymentResult{}, fmt.Errorf("simulated payment failure (20%% chance)")
	}

	transactionID := fmt.Sprintf("txn_%s", uuid.New().String()[:8])
	slog.Info("payment processed", "transactionID", transactionID)

	return ProcessPaymentResult{TransactionID: transactionID}, nil
}

// RefundPaymentInput is the input for RefundPayment.
type RefundPaymentInput struct {
	WorkflowID string
	Amount     float64
}

// RefundPayment reverses a prior payment as a compensation action.
func (a *Activities) RefundPayment(ctx context.Context, input RefundPaymentInput) error {
	activity.RecordHeartbeat(ctx, "refunding payment")
	slog.Info("refunding payment", "workflowID", input.WorkflowID, "amount", input.Amount)
	time.Sleep(1 * time.Second)
	slog.Info("payment refunded", "workflowID", input.WorkflowID)
	return nil
}

// ReserveBookingInput is the input for ReserveBooking.
type ReserveBookingInput struct {
	WorkflowID  string
	GuestName   string
	GuestEmail  string
	Items       []BookingItemInput
	TotalAmount float64
}

// ReserveBooking creates the booking record and its line items in the database.
func (a *Activities) ReserveBooking(ctx context.Context, input ReserveBookingInput) error {
	activity.RecordHeartbeat(ctx, "reserving booking")
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

// SendConfirmationInput is the input for SendConfirmation.
type SendConfirmationInput struct {
	WorkflowID  string
	GuestName   string
	GuestEmail  string
	TotalAmount float64
}

// SendConfirmation simulates sending a confirmation email to the guest.
func (a *Activities) SendConfirmation(ctx context.Context, input SendConfirmationInput) error {
	activity.RecordHeartbeat(ctx, "sending confirmation")
	slog.Info("sending confirmation email", "workflowID", input.WorkflowID, "email", input.GuestEmail)
	time.Sleep(2 * time.Second)
	slog.Info("confirmation email sent", "workflowID", input.WorkflowID)
	return nil
}

// CancelReservationInput is the input for CancelReservation.
type CancelReservationInput struct {
	WorkflowID string
	GuestEmail string
}

// CancelReservation compensates by cancelling the booking reservation.
func (a *Activities) CancelReservation(ctx context.Context, input CancelReservationInput) error {
	activity.RecordHeartbeat(ctx, "cancelling reservation")
	slog.Info("cancelling reservation", "workflowID", input.WorkflowID)

	time.Sleep(1 * time.Second)

	if err := db.CancelBooking(ctx, a.Pool, input.WorkflowID); err != nil {
		return fmt.Errorf("cancelling reservation: %w", err)
	}

	slog.Info("reservation cancelled", "workflowID", input.WorkflowID)
	return nil
}

// SendCancellationEmailInput is the input for SendCancellationEmail.
type SendCancellationEmailInput struct {
	WorkflowID string
	GuestName  string
	GuestEmail string
	Reason     string
}

// SendCancellationEmail simulates sending a cancellation email to the guest.
func (a *Activities) SendCancellationEmail(ctx context.Context, input SendCancellationEmailInput) error {
	activity.RecordHeartbeat(ctx, "sending cancellation email")
	slog.Info("sending cancellation email", "workflowID", input.WorkflowID, "email", input.GuestEmail)

	time.Sleep(1 * time.Second)

	slog.Info("cancellation email sent", "workflowID", input.WorkflowID)
	return nil
}

// DeleteEmbeddingsInput is the input for DeleteEmbeddings.
type DeleteEmbeddingsInput struct {
	Version string
}

// DeleteEmbeddings removes all embeddings for a given version (rollback).
func (a *Activities) DeleteEmbeddings(ctx context.Context, input DeleteEmbeddingsInput) error {
	slog.Info("deleting embeddings", "version", input.Version)

	time.Sleep(1 * time.Second)

	if err := db.DeleteVersionEmbeddings(ctx, a.Pool, input.Version); err != nil {
		return fmt.Errorf("deleting embeddings: %w", err)
	}

	slog.Info("embeddings deleted", "version", input.Version)
	return nil
}
