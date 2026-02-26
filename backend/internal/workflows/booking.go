package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/sakeththota/durable-embedding-migration/backend/internal/activities"
)

const (
	BookingTaskQueue   = "booking-checkout"
	QueryBookingStatus = "booking-status"
)

type BookingInput struct {
	WorkflowID string
	GuestName  string
	GuestEmail string
	Items      []activities.BookingItemInput
}

type BookingProgress struct {
	Status      string    `json:"status"`
	GuestName   string    `json:"guest_name"`
	GuestEmail  string    `json:"guest_email"`
	TotalAmount float64   `json:"total_amount"`
	CurrentStep string    `json:"current_step"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

func BookingCheckoutWorkflow(ctx workflow.Context, input BookingInput) error {
	logger := workflow.GetLogger(ctx)

	progress := BookingProgress{
		Status:      "processing",
		GuestName:   input.GuestName,
		GuestEmail:  input.GuestEmail,
		CurrentStep: "validating_availability",
		StartedAt:   workflow.Now(ctx),
	}

	if err := workflow.SetQueryHandler(ctx, QueryBookingStatus, func() (BookingProgress, error) {
		return progress, nil
	}); err != nil {
		return err
	}

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    3,
		},
	})

	var validateResult activities.ValidateAvailabilityResult
	err := workflow.ExecuteActivity(actCtx, "ValidateAvailability", activities.ValidateAvailabilityInput{
		WorkflowID: input.WorkflowID,
		Items:      input.Items,
	}).Get(ctx, &validateResult)
	if err != nil {
		logger.Error("validation failed", "error", err)
		progress.Status = "failed"
		progress.Error = "validation_failed"
		progress.CurrentStep = "validation_failed"
		return err
	}

	progress.CurrentStep = "processing_payment"
	logger.Info("processing payment", "amount", validateResult.TotalAmount)

	var paymentResult activities.ProcessPaymentResult
	err = workflow.ExecuteActivity(actCtx, "ProcessPayment", activities.ProcessPaymentInput{
		WorkflowID: input.WorkflowID,
		Amount:     validateResult.TotalAmount,
		GuestEmail: input.GuestEmail,
	}).Get(ctx, &paymentResult)
	if err != nil {
		logger.Error("payment failed", "error", err)
		progress.Status = "failed"
		progress.Error = "payment_failed"
		progress.CurrentStep = "payment_failed"

		logger.Info("compensating: refunding payment")
		_ = workflow.ExecuteActivity(actCtx, "RefundPayment", activities.RefundPaymentInput{
			WorkflowID: input.WorkflowID,
			Amount:     validateResult.TotalAmount,
		}).Get(ctx, nil)

		return err
	}

	progress.CurrentStep = "reserving_booking"
	logger.Info("reserving booking")

	err = workflow.ExecuteActivity(actCtx, "ReserveBooking", activities.ReserveBookingInput{
		WorkflowID:  input.WorkflowID,
		GuestName:   input.GuestName,
		GuestEmail:  input.GuestEmail,
		Items:       input.Items,
		TotalAmount: validateResult.TotalAmount,
	}).Get(ctx, nil)
	if err != nil {
		logger.Error("reservation failed", "error", err)
		progress.Status = "failed"
		progress.Error = "reservation_failed"
		progress.CurrentStep = "reservation_failed"

		logger.Info("compensating: refunding payment")
		_ = workflow.ExecuteActivity(actCtx, "RefundPayment", activities.RefundPaymentInput{
			WorkflowID: input.WorkflowID,
			Amount:     validateResult.TotalAmount,
		}).Get(ctx, nil)

		return err
	}

	progress.CurrentStep = "sending_confirmation"
	logger.Info("sending confirmation email")

	err = workflow.ExecuteActivity(actCtx, "SendConfirmation", activities.SendConfirmationInput{
		WorkflowID:  input.WorkflowID,
		GuestName:   input.GuestName,
		GuestEmail:  input.GuestEmail,
		TotalAmount: validateResult.TotalAmount,
	}).Get(ctx, nil)
	if err != nil {
		logger.Info("confirmation email failed (non-critical)", "error", err.Error())
	}

	progress.Status = "completed"
	progress.CurrentStep = "completed"
	progress.TotalAmount = validateResult.TotalAmount
	progress.CompletedAt = workflow.Now(ctx)

	logger.Info("booking completed", "workflowID", input.WorkflowID)
	return nil
}
