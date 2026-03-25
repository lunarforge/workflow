package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lunarforge/workflow"
)

// OrderStatus represents all possible states in the order workflow.
type OrderStatus int

const (
	OrderStatusUnknown           OrderStatus = 0
	OrderStatusCreated           OrderStatus = 1
	OrderStatusValidated         OrderStatus = 2
	OrderStatusPaymentProcessing OrderStatus = 3
	OrderStatusPaymentProcessed  OrderStatus = 4
	OrderStatusPaymentFailed     OrderStatus = 5
	OrderStatusInventoryReserved OrderStatus = 6
	OrderStatusInventoryFailed   OrderStatus = 7
	OrderStatusFulfilled         OrderStatus = 8
	OrderStatusCancelled         OrderStatus = 9
	OrderStatusRefunded          OrderStatus = 10
)

func (s OrderStatus) String() string {
	switch s {
	case OrderStatusCreated:
		return "Created"
	case OrderStatusValidated:
		return "Validated"
	case OrderStatusPaymentProcessing:
		return "PaymentProcessing"
	case OrderStatusPaymentProcessed:
		return "PaymentProcessed"
	case OrderStatusPaymentFailed:
		return "PaymentFailed"
	case OrderStatusInventoryReserved:
		return "InventoryReserved"
	case OrderStatusInventoryFailed:
		return "InventoryFailed"
	case OrderStatusFulfilled:
		return "Fulfilled"
	case OrderStatusCancelled:
		return "Cancelled"
	case OrderStatusRefunded:
		return "Refunded"
	default:
		return "Unknown"
	}
}

// Order is the business entity flowing through the workflow.
type Order struct {
	ID              string      `json:"id"`
	CustomerID      string      `json:"customer_id"`
	Items           []OrderItem `json:"items"`
	Total           float64     `json:"total"`
	PaymentMethod   string      `json:"payment_method"`
	ShippingAddress Address     `json:"shipping_address"`

	ValidationErrors []string   `json:"validation_errors,omitempty"`
	PaymentID        string     `json:"payment_id,omitempty"`
	ReservationID    string     `json:"reservation_id,omitempty"`
	TrackingNumber   string     `json:"tracking_number,omitempty"`
	PaymentAttempts  int        `json:"payment_attempts"`
	CreatedAt        time.Time  `json:"created_at"`
	FulfilledAt      *time.Time `json:"fulfilled_at,omitempty"`
}

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	ZipCode string `json:"zip_code"`
	Country string `json:"country"`
}

// Step functions

func validateOrder(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	time.Sleep(200 * time.Millisecond)

	if len(r.Object.Items) == 0 {
		r.Object.ValidationErrors = []string{"Order must have at least one item"}
		return OrderStatusCancelled, nil
	}
	if r.Object.ShippingAddress.Street == "" {
		r.Object.ValidationErrors = []string{"Shipping address is required"}
		return OrderStatusCancelled, nil
	}

	return OrderStatusValidated, nil
}

func processPayment(ps *MockPaymentService) func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	return func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
		time.Sleep(300 * time.Millisecond)
		r.Object.PaymentAttempts++

		paymentID, err := ps.ProcessPayment(r.Object.Total, r.Object.PaymentMethod, r.Object.ID)
		if err != nil {
			if r.Object.PaymentAttempts < 3 {
				return r.SaveAndRepeat()
			}
			return OrderStatusPaymentFailed, nil
		}

		r.Object.PaymentID = paymentID
		return OrderStatusPaymentProcessing, nil
	}
}

func completePayment(_ *MockPaymentService) func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	return func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
		time.Sleep(200 * time.Millisecond)
		return OrderStatusPaymentProcessed, nil
	}
}

func reserveInventory(is *MockInventoryService) func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	return func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
		time.Sleep(250 * time.Millisecond)

		reservationID, err := is.ReserveItems(r.Object.Items)
		if err != nil {
			return OrderStatusInventoryFailed, nil
		}

		r.Object.ReservationID = reservationID
		return OrderStatusInventoryReserved, nil
	}
}

func fulfillOrder(_ *MockShippingService) func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	return func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
		time.Sleep(300 * time.Millisecond)

		r.Object.TrackingNumber = fmt.Sprintf("TRACK_%s", r.Object.ID)
		now := time.Now()
		r.Object.FulfilledAt = &now
		return OrderStatusFulfilled, nil
	}
}

func handlePaymentFailure(_ *MockPaymentService) func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	return func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
		return OrderStatusCancelled, nil
	}
}

func handleInventoryFailure(_ *MockInventoryService, ps *MockPaymentService) func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	return func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
		_ = ps.RefundPayment(r.Object.PaymentID)
		return OrderStatusRefunded, nil
	}
}

func processCancellation(is *MockInventoryService, ps *MockPaymentService) func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
	return func(ctx context.Context, r *workflow.Run[Order, OrderStatus]) (OrderStatus, error) {
		if r.Object.ReservationID != "" {
			_ = is.ReleaseReservation(r.Object.ReservationID)
		}
		if r.Object.PaymentID != "" {
			_ = ps.RefundPayment(r.Object.PaymentID)
		}
		return OrderStatusRefunded, nil
	}
}

// Mock services

type MockPaymentService struct{}

func (s *MockPaymentService) ProcessPayment(amount float64, method, orderID string) (string, error) {
	if amount > 1000 && method == "credit_card" {
		return "", errors.New("payment declined by bank")
	}
	return fmt.Sprintf("pay_%s", orderID), nil
}

func (s *MockPaymentService) RefundPayment(paymentID string) error {
	return nil
}

type MockInventoryService struct {
	mu      sync.Mutex
	counter int
}

func (s *MockInventoryService) ReserveItems(items []OrderItem) (string, error) {
	for _, item := range items {
		if item.ProductID == "out_of_stock" {
			return "", errors.New("insufficient inventory")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	return fmt.Sprintf("res_%d", s.counter), nil
}

func (s *MockInventoryService) ReleaseReservation(_ string) error {
	return nil
}

type MockShippingService struct{}
