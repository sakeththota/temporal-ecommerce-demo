package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/sakeththota/temporal-ecommerce-demo/backend/internal/activities"
)

const (
	BookingTaskQueue    = "booking-checkout"
	QueryBookingStatus  = "booking-status"
	SignalCancelBooking = "cancel-booking"
)

type BookingInput struct {
	WorkflowID string
	GuestName  string
	GuestEmail string
	Items      []activities.BookingItemInput
}

type BookingProgress struct {
	Status           string    `json:"status"`
	GuestName        string    `json:"guest_name"`
	GuestEmail       string    `json:"guest_email"`
	TotalAmount      float64   `json:"total_amount"`
	CurrentStep      string    `json:"current_step"`
	Error            string    `json:"error,omitempty"`
	CompensationRun  bool      `json:"compensation_run"`
	CompensationStep string    `json:"compensation_step,omitempty"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at,omitempty"`
}

type BookingUpdateInput struct {
	Items []activities.BookingItemInput
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

	paymentProcessed := false
	reservationMade := false

	_ = paymentProcessed // track for potential cancel logic
	_ = reservationMade  // track for potential cancel logic

	if err := workflow.SetQueryHandler(ctx, QueryBookingStatus, func() (BookingProgress, error) {
		return progress, nil
	}); err != nil {
		return err
	}

	signalCh := workflow.GetSignalChannel(ctx, SignalCancelBooking)

	var cancelSignal struct{}
	workflow.Go(ctx, func(ctx workflow.Context) {
		signalCh.Receive(ctx, &cancelSignal)
		workflow.GetLogger(ctx).Info("cancel signal received")
	})

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    3,
		},
	})

	logger.Info("booking workflow started", "workflowID", input.WorkflowID, "guest", input.GuestName)

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

	progress.TotalAmount = validateResult.TotalAmount
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
		return err
	}
	paymentProcessed = true
	logger.Info("payment successful", "transactionID", paymentResult.TransactionID)

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
		progress.CompensationRun = true
		progress.CompensationStep = "refunding_payment"

		refundErr := workflow.ExecuteActivity(actCtx, "RefundPayment", activities.RefundPaymentInput{
			WorkflowID: input.WorkflowID,
			Amount:     validateResult.TotalAmount,
		}).Get(ctx, nil)
		if refundErr != nil {
			logger.Error("refund failed", "error", refundErr)
			progress.CompensationStep = "refund_failed"
		} else {
			logger.Info("payment refunded successfully")
			progress.CompensationStep = "payment_refunded"
		}

		return err
	}
	reservationMade = true

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
	progress.CompletedAt = workflow.Now(ctx)

	logger.Info("booking completed", "workflowID", input.WorkflowID)
	return nil
}

func BookingCompensationDemoWorkflow(ctx workflow.Context, input BookingInput) error {
	logger := workflow.GetLogger(ctx)

	progress := BookingProgress{
		Status:      "processing",
		GuestName:   input.GuestName,
		GuestEmail:  input.GuestEmail,
		CurrentStep: "validating_availability",
		StartedAt:   workflow.Now(ctx),
	}

	paymentProcessed := false

	_ = paymentProcessed // track for potential cancel logic

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

	logger.Info("booking compensation demo started", "workflowID", input.WorkflowID, "guest", input.GuestName)

	var validateResult activities.ValidateAvailabilityResult
	err := workflow.ExecuteActivity(actCtx, "ValidateAvailability", activities.ValidateAvailabilityInput{
		WorkflowID: input.WorkflowID,
		Items:      input.Items,
	}).Get(ctx, &validateResult)
	if err != nil {
		logger.Error("validation failed", "error", err)
		progress.Status = "failed"
		progress.Error = "validation_failed"
		return err
	}

	progress.TotalAmount = validateResult.TotalAmount
	progress.CurrentStep = "processing_payment"

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
		return err
	}
	paymentProcessed = true
	logger.Info("payment successful", "transactionID", paymentResult.TransactionID)

	progress.CurrentStep = "reserving_booking"

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
		progress.CompensationRun = true
		progress.CompensationStep = "refunding_payment"

		_ = workflow.ExecuteActivity(actCtx, "RefundPayment", activities.RefundPaymentInput{
			WorkflowID: input.WorkflowID,
			Amount:     validateResult.TotalAmount,
		}).Get(ctx, nil)

		return err
	}

	progress.CurrentStep = "sending_confirmation"

	err = workflow.ExecuteActivity(actCtx, "SendConfirmation", activities.SendConfirmationInput{
		WorkflowID:  input.WorkflowID,
		GuestName:   input.GuestName,
		GuestEmail:  input.GuestEmail,
		TotalAmount: validateResult.TotalAmount,
	}).Get(ctx, nil)
	if err != nil {
		logger.Info("confirmation email failed (non-critical)", "error", err.Error())
		progress.CompensationRun = true
		progress.CompensationStep = "confirmation_failed_compensating"

		logger.Info("compensating: cancelling reservation and refunding")
		_ = workflow.ExecuteActivity(actCtx, "CancelReservation", activities.CancelReservationInput{
			WorkflowID: input.WorkflowID,
			GuestEmail: input.GuestEmail,
		}).Get(ctx, nil)

		_ = workflow.ExecuteActivity(actCtx, "RefundPayment", activities.RefundPaymentInput{
			WorkflowID: input.WorkflowID,
			Amount:     validateResult.TotalAmount,
		}).Get(ctx, nil)

		progress.CompensationStep = "fully_compensated"
		progress.Status = "cancelled"
		progress.CompletedAt = workflow.Now(ctx)
		return nil
	}

	progress.Status = "completed"
	progress.CurrentStep = "completed"
	progress.CompletedAt = workflow.Now(ctx)

	logger.Info("booking completed", "workflowID", input.WorkflowID)
	return nil
}
