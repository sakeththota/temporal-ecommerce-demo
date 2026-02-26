package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Booking struct {
	ID          uuid.UUID `json:"id"`
	WorkflowID  string    `json:"workflow_id"`
	GuestName   string    `json:"guest_name"`
	GuestEmail  string    `json:"guest_email"`
	TotalAmount float64   `json:"total_amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BookingItem struct {
	ID            uuid.UUID `json:"id"`
	BookingID     uuid.UUID `json:"booking_id"`
	HotelID       uuid.UUID `json:"hotel_id"`
	CheckIn       time.Time `json:"check_in"`
	CheckOut      time.Time `json:"check_out"`
	Nights        int       `json:"nights"`
	PricePerNight float64   `json:"price_per_night"`
	Subtotal      float64   `json:"subtotal"`
	CreatedAt     time.Time `json:"created_at"`
}

type BookingWithItems struct {
	Booking Booking       `json:"booking"`
	Items   []BookingItem `json:"items"`
}

func CreateBooking(ctx context.Context, pool *pgxpool.Pool, workflowID, guestName, guestEmail string, totalAmount float64) (uuid.UUID, error) {
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO bookings (workflow_id, guest_name, guest_email, total_amount, status, updated_at)
		 VALUES ($1, $2, $3, $4, 'pending', NOW())
		 RETURNING id`,
		workflowID, guestName, guestEmail, totalAmount).Scan(&id)
	return id, err
}

func GetBookingByWorkflowID(ctx context.Context, pool *pgxpool.Pool, workflowID string) (*BookingWithItems, error) {
	var booking Booking
	err := pool.QueryRow(ctx,
		`SELECT id, workflow_id, guest_name, guest_email, total_amount, status, created_at, updated_at
		 FROM bookings WHERE workflow_id = $1`, workflowID).Scan(
		&booking.ID, &booking.WorkflowID, &booking.GuestName, &booking.GuestEmail,
		&booking.TotalAmount, &booking.Status, &booking.CreatedAt, &booking.UpdatedAt)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx,
		`SELECT id, booking_id, hotel_id, check_in, check_out, nights, price_per_night, subtotal, created_at
		 FROM booking_items WHERE booking_id = $1`, booking.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[BookingItem])
	if err != nil {
		return nil, err
	}

	return &BookingWithItems{Booking: booking, Items: items}, nil
}

func AddBookingItem(ctx context.Context, pool *pgxpool.Pool, bookingID, hotelID uuid.UUID, checkIn, checkOut time.Time, nights int, pricePerNight float64) error {
	_, err := pool.Exec(ctx,
		`INSERT INTO booking_items (booking_id, hotel_id, check_in, check_out, nights, price_per_night, subtotal)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		bookingID, hotelID, checkIn, checkOut, nights, pricePerNight, float64(nights)*pricePerNight)
	return err
}

func UpdateBookingStatus(ctx context.Context, pool *pgxpool.Pool, workflowID, status string) error {
	_, err := pool.Exec(ctx,
		`UPDATE bookings SET status = $1, updated_at = NOW() WHERE workflow_id = $2`,
		status, workflowID)
	return err
}

func ListBookings(ctx context.Context, pool *pgxpool.Pool) ([]Booking, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, workflow_id, guest_name, guest_email, total_amount, status, created_at, updated_at
		 FROM bookings ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowToStructByPos[Booking])
}
