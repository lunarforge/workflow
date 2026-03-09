// Command flowwatch-demo starts an order-processing workflow with the FlowWatch
// monitoring dashboard. It triggers sample orders with mixed outcomes (success,
// payment failure, inventory failure) and serves the Connect-Go API on :8090.
//
// Usage:
//
//	go run .
//
// Then start the UI in a separate terminal:
//
//	cd flowwatch/ui && VITE_API_URL=http://localhost:8090 bun run dev
//
// Open http://localhost:5173 to view the dashboard.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/memrecordstore"
	"github.com/lunarforge/workflow/adapters/memrolescheduler"
	"github.com/lunarforge/workflow/adapters/memstepstore"
	"github.com/lunarforge/workflow/adapters/memstreamer"
	"github.com/lunarforge/workflow/adapters/memtimeoutstore"

	"github.com/lunarforge/workflow/flowwatch"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.Kitchen}).
		With().Timestamp().Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- Workflow infrastructure ---
	recordStore := memrecordstore.New()
	streamer := memstreamer.New()
	stepStore := memstepstore.New()

	paymentSvc := &MockPaymentService{}
	inventorySvc := &MockInventoryService{}
	shippingSvc := &MockShippingService{}

	wf := buildWorkflow(streamer, recordStore, stepStore, paymentSvc, inventorySvc, shippingSvc)
	onboardingWf := buildOnboardingWorkflow(streamer, recordStore, stepStore)
	refundWf := buildRefundWorkflow(streamer, recordStore, stepStore)

	wf.Run(ctx)
	defer wf.Stop()
	onboardingWf.Run(ctx)
	defer onboardingWf.Stop()
	refundWf.Run(ctx)
	defer refundWf.Stop()

	// --- FlowWatch adapter ---
	adapter := flowwatch.NewAdapter(recordStore, stepStore)
	flowwatch.RegisterWorkflow(adapter, wf, flowwatch.WithSubsystem("order-fulfillment"))
	flowwatch.RegisterWorkflow(adapter, onboardingWf, flowwatch.WithSubsystem("identity"))
	flowwatch.RegisterWorkflow(adapter, refundWf, flowwatch.WithSubsystem("billing"))

	// --- FlowWatch API server (with analytics caching) ---
	mux := http.NewServeMux()
	stopCache := flowwatch.RegisterHandlersWithCache(ctx, mux, adapter)
	defer stopCache()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	handler := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173"},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	}).Handler(mux)

	addr := ":8090"
	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(handler, &http2.Server{}),
	}

	go func() {
		fmt.Printf("FlowWatch API running on http://localhost%s\n", addr)
		fmt.Println("Start the UI:  cd flowwatch/ui && VITE_API_URL=http://localhost:8090 bun run dev")
		fmt.Println("Dashboard:     http://localhost:5173")
		fmt.Println()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	// --- Trigger sample data ---
	triggerSampleOrders(ctx, wf)
	triggerSampleOnboarding(ctx, onboardingWf)
	triggerSampleRefunds(ctx, refundWf)

	// --- Start continuous spawner (realistic load curve) ---
	startSpawner(ctx, workflows{
		order:      wf,
		onboarding: onboardingWf,
		refund:     refundWf,
	})

	<-ctx.Done()
	fmt.Println("\nShutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func buildWorkflow(
	streamer *memstreamer.StreamConstructor,
	recordStore *memrecordstore.Store,
	stepStore *memstepstore.Store,
	ps *MockPaymentService,
	is *MockInventoryService,
	ss *MockShippingService,
) *workflow.Workflow[Order, OrderStatus] {
	b := workflow.NewBuilder[Order, OrderStatus]("order-processing")

	b.AddStep(OrderStatusCreated, validateOrder, OrderStatusValidated, OrderStatusCancelled)
	b.AddStep(OrderStatusValidated, processPayment(ps), OrderStatusPaymentProcessing, OrderStatusPaymentFailed, OrderStatusCancelled)
	b.AddStep(OrderStatusPaymentProcessing, completePayment(ps), OrderStatusPaymentProcessed, OrderStatusPaymentFailed)
	b.AddStep(OrderStatusPaymentProcessed, reserveInventory(is), OrderStatusInventoryReserved, OrderStatusInventoryFailed)
	b.AddStep(OrderStatusInventoryReserved, fulfillOrder(ss), OrderStatusFulfilled)
	b.AddStep(OrderStatusPaymentFailed, handlePaymentFailure(ps), OrderStatusCancelled, OrderStatusPaymentProcessing)
	b.AddStep(OrderStatusInventoryFailed, handleInventoryFailure(is, ps), OrderStatusRefunded, OrderStatusInventoryReserved)
	b.AddStep(OrderStatusCancelled, processCancellation(is, ps), OrderStatusRefunded)

	b.AddTimeout(
		OrderStatusPaymentProcessing,
		workflow.DurationTimerFunc[Order, OrderStatus](5*time.Minute),
		func(ctx context.Context, r *workflow.Run[Order, OrderStatus], now time.Time) (OrderStatus, error) {
			return OrderStatusPaymentFailed, nil
		},
		OrderStatusPaymentFailed,
	)

	return b.Build(
		streamer,
		recordStore,
		memrolescheduler.New(),
		workflow.WithTimeoutStore(memtimeoutstore.New()),
		workflow.WithStepStore(stepStore),
	)
}

func triggerSampleOrders(ctx context.Context, wf *workflow.Workflow[Order, OrderStatus]) {
	orders := []Order{
		{
			// Happy path: succeeds end-to-end.
			ID:            "order-001",
			CustomerID:    "customer-123",
			Total:         99.99,
			PaymentMethod: "credit_card",
			Items:         []OrderItem{{ProductID: "laptop-stand", Quantity: 1, Price: 99.99}},
			ShippingAddress: Address{
				Street: "123 Main St", City: "San Francisco", State: "CA", ZipCode: "94105", Country: "USA",
			},
			CreatedAt: time.Now(),
		},
		{
			// Payment failure: total > 1000 with credit_card triggers decline.
			ID:            "order-002",
			CustomerID:    "customer-789",
			Total:         1500.00,
			PaymentMethod: "credit_card",
			Items:         []OrderItem{{ProductID: "gaming-desktop", Quantity: 1, Price: 1500.00}},
			ShippingAddress: Address{
				Street: "456 Tech Ave", City: "Austin", State: "TX", ZipCode: "78701", Country: "USA",
			},
			CreatedAt: time.Now(),
		},
		{
			// Inventory failure: "out_of_stock" product triggers insufficient inventory.
			ID:            "order-003",
			CustomerID:    "customer-456",
			Total:         49.99,
			PaymentMethod: "credit_card",
			Items:         []OrderItem{{ProductID: "out_of_stock", Quantity: 1, Price: 49.99}},
			ShippingAddress: Address{
				Street: "789 Commerce Blvd", City: "Seattle", State: "WA", ZipCode: "98101", Country: "USA",
			},
			CreatedAt: time.Now(),
		},
	}

	for _, order := range orders {
		fmt.Printf("Triggering order %s (%s, $%.2f)\n", order.ID, order.Items[0].ProductID, order.Total)
		_, err := wf.Trigger(ctx, order.ID, workflow.WithInitialValue[Order, OrderStatus](&order))
		if err != nil {
			fmt.Fprintf(os.Stderr, "trigger error for %s: %v\n", order.ID, err)
		}
	}
}
