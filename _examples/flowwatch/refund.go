package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

// RefundStatus represents states in the refund workflow.
type RefundStatus int

const (
	RefundStatusUnknown           RefundStatus = 0
	RefundStatusRequested         RefundStatus = 1
	RefundStatusValidating        RefundStatus = 2
	RefundStatusPolicyChecked     RefundStatus = 3
	RefundStatusPolicyRejected    RefundStatus = 4
	RefundStatusProcessing        RefundStatus = 5
	RefundStatusAwaitingApproval  RefundStatus = 6
	RefundStatusApproved          RefundStatus = 7
	RefundStatusRejected          RefundStatus = 8
	RefundStatusRefunded          RefundStatus = 9
	RefundStatusNotifyingCustomer RefundStatus = 10
	RefundStatusCompleted         RefundStatus = 11
)

func (s RefundStatus) String() string {
	switch s {
	case RefundStatusRequested:
		return "Requested"
	case RefundStatusValidating:
		return "Validating"
	case RefundStatusPolicyChecked:
		return "PolicyChecked"
	case RefundStatusPolicyRejected:
		return "PolicyRejected"
	case RefundStatusProcessing:
		return "Processing"
	case RefundStatusAwaitingApproval:
		return "AwaitingApproval"
	case RefundStatusApproved:
		return "Approved"
	case RefundStatusRejected:
		return "Rejected"
	case RefundStatusRefunded:
		return "Refunded"
	case RefundStatusNotifyingCustomer:
		return "NotifyingCustomer"
	case RefundStatusCompleted:
		return "Completed"
	default:
		return "Unknown"
	}
}

// RefundRequest is the business entity for the refund workflow.
type RefundRequest struct {
	RefundID    string    `json:"refund_id"`
	OrderID     string    `json:"order_id"`
	CustomerID  string    `json:"customer_id"`
	Amount      float64   `json:"amount"`
	Reason      string    `json:"reason"`
	ItemIDs     []string  `json:"item_ids"`
	ApproverID  string    `json:"approver_id,omitempty"`
	PaymentRef  string    `json:"payment_ref,omitempty"`
	PolicyMatch string    `json:"policy_match,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func buildRefundWorkflow(
	streamer *memstreamer.StreamConstructor,
	recordStore *memrecordstore.Store,
	stepStore workflow.StepStore,
) *workflow.Workflow[RefundRequest, RefundStatus] {
	b := workflow.NewBuilder[RefundRequest, RefundStatus]("refund-process")

	b.AddStep(RefundStatusRequested, validateRefund, RefundStatusValidating, RefundStatusRejected)
	b.AddStep(RefundStatusValidating, checkPolicy, RefundStatusPolicyChecked, RefundStatusPolicyRejected)
	b.AddStep(RefundStatusPolicyChecked, routeRefund, RefundStatusProcessing, RefundStatusAwaitingApproval)
	b.AddStep(RefundStatusAwaitingApproval, approveRefund, RefundStatusApproved, RefundStatusRejected)
	b.AddStep(RefundStatusApproved, processRefund, RefundStatusRefunded)
	b.AddStep(RefundStatusProcessing, processRefund, RefundStatusRefunded)
	b.AddStep(RefundStatusRefunded, notifyCustomer, RefundStatusNotifyingCustomer)
	b.AddStep(RefundStatusNotifyingCustomer, completeRefund, RefundStatusCompleted)
	b.AddStep(RefundStatusPolicyRejected, handlePolicyRejection, RefundStatusRejected, RefundStatusAwaitingApproval)

	return b.Build(
		streamer,
		recordStore,
		memrolescheduler.New(),
		workflow.WithStepStore(stepStore),
	)
}

func validateRefund(_ context.Context, r *workflow.Run[RefundRequest, RefundStatus]) (RefundStatus, error) {
	time.Sleep(100 * time.Millisecond)
	if r.Object.Amount <= 0 {
		return RefundStatusRejected, nil
	}
	if len(r.Object.ItemIDs) == 0 {
		return RefundStatusRejected, nil
	}
	return RefundStatusValidating, nil
}

func checkPolicy(_ context.Context, r *workflow.Run[RefundRequest, RefundStatus]) (RefundStatus, error) {
	time.Sleep(200 * time.Millisecond)
	// Simulate policy check: reject "final_sale" reason
	if r.Object.Reason == "final_sale" {
		r.Object.PolicyMatch = "FINAL_SALE_NO_REFUND"
		return RefundStatusPolicyRejected, nil
	}
	r.Object.PolicyMatch = "STANDARD_RETURN"
	return RefundStatusPolicyChecked, nil
}

func routeRefund(_ context.Context, r *workflow.Run[RefundRequest, RefundStatus]) (RefundStatus, error) {
	time.Sleep(50 * time.Millisecond)
	// High-value refunds need manual approval
	if r.Object.Amount > 500 {
		return RefundStatusAwaitingApproval, nil
	}
	return RefundStatusProcessing, nil
}

func approveRefund(_ context.Context, r *workflow.Run[RefundRequest, RefundStatus]) (RefundStatus, error) {
	time.Sleep(300 * time.Millisecond)
	// Simulate: reject refunds over $5000
	if r.Object.Amount > 5000 {
		return RefundStatusRejected, nil
	}
	r.Object.ApproverID = "manager-auto"
	return RefundStatusApproved, nil
}

func processRefund(_ context.Context, r *workflow.Run[RefundRequest, RefundStatus]) (RefundStatus, error) {
	time.Sleep(250 * time.Millisecond)
	r.Object.PaymentRef = fmt.Sprintf("refund_%s", r.Object.RefundID)
	return RefundStatusRefunded, nil
}

func notifyCustomer(_ context.Context, _ *workflow.Run[RefundRequest, RefundStatus]) (RefundStatus, error) {
	time.Sleep(100 * time.Millisecond)
	return RefundStatusNotifyingCustomer, nil
}

func completeRefund(_ context.Context, _ *workflow.Run[RefundRequest, RefundStatus]) (RefundStatus, error) {
	time.Sleep(50 * time.Millisecond)
	return RefundStatusCompleted, nil
}

func handlePolicyRejection(_ context.Context, r *workflow.Run[RefundRequest, RefundStatus]) (RefundStatus, error) {
	time.Sleep(100 * time.Millisecond)
	// Escalate to manual review if policy rejected but reason seems valid
	if r.Object.Reason == "defective" {
		return RefundStatusAwaitingApproval, nil
	}
	return RefundStatusRejected, nil
}

// MockRefundGateway simulates a refund payment processor.
type MockRefundGateway struct{}

func (g *MockRefundGateway) IssueRefund(amount float64, orderID string) (string, error) {
	if amount > 10000 {
		return "", errors.New("refund exceeds maximum limit")
	}
	return fmt.Sprintf("ref_%s", orderID), nil
}

func triggerSampleRefunds(ctx context.Context, wf *workflow.Workflow[RefundRequest, RefundStatus]) {
	refunds := []RefundRequest{
		{
			// Auto-approved: small amount, standard return.
			RefundID:   "refund-001",
			OrderID:    "order-101",
			CustomerID: "customer-123",
			Amount:     29.99,
			Reason:     "wrong_size",
			ItemIDs:    []string{"item-a"},
			CreatedAt:  time.Now(),
		},
		{
			// Manual approval: amount > $500 requires manager review.
			RefundID:   "refund-002",
			OrderID:    "order-102",
			CustomerID: "customer-456",
			Amount:     899.00,
			Reason:     "defective",
			ItemIDs:    []string{"item-b", "item-c"},
			CreatedAt:  time.Now(),
		},
		{
			// Policy rejected: "final_sale" reason triggers policy rejection.
			RefundID:   "refund-003",
			OrderID:    "order-103",
			CustomerID: "customer-789",
			Amount:     149.99,
			Reason:     "final_sale",
			ItemIDs:    []string{"item-d"},
			CreatedAt:  time.Now(),
		},
	}

	for _, r := range refunds {
		fmt.Printf("Triggering refund %s (order %s, $%.2f, %s)\n", r.RefundID, r.OrderID, r.Amount, r.Reason)
		_, err := wf.Trigger(ctx, r.RefundID, workflow.WithInitialValue[RefundRequest, RefundStatus](&r))
		if err != nil {
			fmt.Fprintf(os.Stderr, "trigger error for %s: %v\n", r.RefundID, err)
		}
	}
}
