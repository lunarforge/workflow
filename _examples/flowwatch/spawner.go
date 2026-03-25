package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/lunarforge/workflow"
)

// spawnConfig controls the load curve shape.
type spawnConfig struct {
	// Load curve phases (durations).
	RampUp   time.Duration // Ramp from minRate to maxRate.
	Plateau  time.Duration // Stay at maxRate.
	CoolDown time.Duration // Ramp back down to minRate.

	// Runs per second at each phase.
	MinRate float64
	MaxRate float64
}

var defaultSpawnConfig = spawnConfig{
	RampUp:   30 * time.Second,
	Plateau:  2 * time.Minute,
	CoolDown: 30 * time.Second,
	MinRate:  0.2, // 1 every 5 seconds
	MaxRate:  2.0, // 2 per second
}

// currentRate computes the target runs/second at a given elapsed time using
// a ramp-up → plateau → cool-down curve that repeats.
func (c spawnConfig) currentRate(elapsed time.Duration) float64 {
	cycle := c.RampUp + c.Plateau + c.CoolDown
	t := elapsed % cycle

	switch {
	case t < c.RampUp:
		// Linear ramp up.
		frac := float64(t) / float64(c.RampUp)
		return c.MinRate + (c.MaxRate-c.MinRate)*frac
	case t < c.RampUp+c.Plateau:
		return c.MaxRate
	default:
		// Linear cool down.
		frac := float64(t-(c.RampUp+c.Plateau)) / float64(c.CoolDown)
		return c.MaxRate - (c.MaxRate-c.MinRate)*frac
	}
}

type workflows struct {
	order      *workflow.Workflow[Order, OrderStatus]
	onboarding *workflow.Workflow[UserOnboarding, OnboardingStatus]
	refund     *workflow.Workflow[RefundRequest, RefundStatus]
}

// startSpawner launches background goroutines that continuously spawn random
// workflow runs following a realistic load curve.
func startSpawner(ctx context.Context, wfs workflows) {
	cfg := defaultSpawnConfig
	start := time.Now()

	go func() {
		var seq int
		for {
			rate := cfg.currentRate(time.Since(start))
			if rate <= 0 {
				rate = cfg.MinRate
			}
			// Sleep interval = 1/rate, with some jitter (±30%).
			base := time.Duration(float64(time.Second) / rate)
			jitter := time.Duration(float64(base) * (rand.Float64()*0.6 - 0.3))
			delay := base + jitter
			if delay < 50*time.Millisecond {
				delay = 50 * time.Millisecond
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}

			seq++
			spawnRandom(ctx, wfs, seq)
		}
	}()

	log.Info().
		Dur("ramp_up", cfg.RampUp).
		Dur("plateau", cfg.Plateau).
		Dur("cool_down", cfg.CoolDown).
		Float64("min_rate", cfg.MinRate).
		Float64("max_rate", cfg.MaxRate).
		Msg("spawner.started")
}

// spawnRandom picks a random workflow type and triggers a run with randomized
// data that produces varied outcomes.
func spawnRandom(ctx context.Context, wfs workflows, seq int) {
	// Weight distribution: 50% orders, 30% onboarding, 20% refunds.
	r := rand.Float64()
	switch {
	case r < 0.50:
		spawnRandomOrder(ctx, wfs.order, seq)
	case r < 0.80:
		spawnRandomOnboarding(ctx, wfs.onboarding, seq)
	default:
		spawnRandomRefund(ctx, wfs.refund, seq)
	}
}

// --- Order spawner ---

var (
	products = []struct {
		id    string
		price float64
	}{
		{"laptop-stand", 49.99},
		{"wireless-mouse", 29.99},
		{"usb-hub", 39.99},
		{"monitor-arm", 89.99},
		{"keyboard", 129.99},
		{"webcam-hd", 79.99},
		{"desk-lamp", 34.99},
		{"cable-organizer", 14.99},
		{"out_of_stock", 19.99}, // Triggers inventory failure
	}
	cities = []struct {
		city, state, zip string
	}{
		{"San Francisco", "CA", "94105"},
		{"Austin", "TX", "78701"},
		{"Seattle", "WA", "98101"},
		{"New York", "NY", "10001"},
		{"Chicago", "IL", "60601"},
		{"Denver", "CO", "80202"},
		{"Portland", "OR", "97201"},
		{"Miami", "FL", "33101"},
	}
)

func spawnRandomOrder(ctx context.Context, wf *workflow.Workflow[Order, OrderStatus], seq int) {
	id := fmt.Sprintf("order-auto-%d", seq)

	// Pick 1-3 random products.
	numItems := rand.Intn(3) + 1
	var items []OrderItem
	var total float64
	for i := 0; i < numItems; i++ {
		p := products[rand.Intn(len(products))]
		qty := rand.Intn(2) + 1
		items = append(items, OrderItem{ProductID: p.id, Quantity: qty, Price: p.price})
		total += p.price * float64(qty)
	}

	// ~15% chance of high-value order (triggers payment failure for credit_card > 1000).
	if rand.Float64() < 0.15 {
		total = math.Round((rand.Float64()*2000+1000)*100) / 100
	}

	// Payment method: mostly credit_card, some debit.
	method := "credit_card"
	if rand.Float64() < 0.3 {
		method = "debit_card"
	}

	city := cities[rand.Intn(len(cities))]

	order := Order{
		ID:            id,
		CustomerID:    fmt.Sprintf("cust-%d", rand.Intn(500)+1),
		Items:         items,
		Total:         math.Round(total*100) / 100,
		PaymentMethod: method,
		ShippingAddress: Address{
			Street:  fmt.Sprintf("%d Commerce St", rand.Intn(9000)+100),
			City:    city.city,
			State:   city.state,
			ZipCode: city.zip,
			Country: "USA",
		},
		CreatedAt: time.Now(),
	}

	_, err := wf.Trigger(ctx, id, workflow.WithInitialValue[Order, OrderStatus](&order))
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("spawner.order.trigger_failed")
	}
}

// --- Onboarding spawner ---

var (
	emailDomains = []string{"gmail.com", "outlook.com", "company.io", "fastmail.com", "proton.me"}
	countries    = []string{"US", "GB", "DE", "FR", "AU", "CA", "JP", "blocked-country"}
	firstNames   = []string{"alice", "bob", "charlie", "diana", "eve", "frank", "grace", "hank", "iris", "jack"}
)

func spawnRandomOnboarding(ctx context.Context, wf *workflow.Workflow[UserOnboarding, OnboardingStatus], seq int) {
	id := fmt.Sprintf("user-auto-%d", seq)
	name := firstNames[rand.Intn(len(firstNames))]
	domain := emailDomains[rand.Intn(len(emailDomains))]
	country := countries[rand.Intn(len(countries))]

	email := fmt.Sprintf("%s%d@%s", name, seq, domain)

	// ~5% chance of empty email (triggers abandoned).
	if rand.Float64() < 0.05 {
		email = ""
	}

	user := UserOnboarding{
		UserID:    id,
		Email:     email,
		Country:   country,
		CreatedAt: time.Now(),
	}

	_, err := wf.Trigger(ctx, id, workflow.WithInitialValue[UserOnboarding, OnboardingStatus](&user))
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("spawner.onboarding.trigger_failed")
	}
}

// --- Refund spawner ---

var (
	refundReasons = []string{"wrong_size", "defective", "not_as_described", "changed_mind", "late_delivery", "final_sale"}
)

func spawnRandomRefund(ctx context.Context, wf *workflow.Workflow[RefundRequest, RefundStatus], seq int) {
	id := fmt.Sprintf("refund-auto-%d", seq)

	// Random amount: mostly small, ~15% large (>500, needs approval), ~5% huge (>5000, gets rejected).
	amount := math.Round((rand.Float64()*200+10)*100) / 100
	if rand.Float64() < 0.15 {
		amount = math.Round((rand.Float64()*2000+500)*100) / 100
	}
	if rand.Float64() < 0.05 {
		amount = math.Round((rand.Float64()*5000+5000)*100) / 100
	}

	reason := refundReasons[rand.Intn(len(refundReasons))]

	numItems := rand.Intn(3) + 1
	var itemIDs []string
	for i := 0; i < numItems; i++ {
		itemIDs = append(itemIDs, fmt.Sprintf("item-%d-%d", seq, i))
	}

	refund := RefundRequest{
		RefundID:   id,
		OrderID:    fmt.Sprintf("order-%d", rand.Intn(1000)+100),
		CustomerID: fmt.Sprintf("cust-%d", rand.Intn(500)+1),
		Amount:     amount,
		Reason:     reason,
		ItemIDs:    itemIDs,
		CreatedAt:  time.Now(),
	}

	_, err := wf.Trigger(ctx, id, workflow.WithInitialValue[RefundRequest, RefundStatus](&refund))
	if err != nil {
		log.Warn().Err(err).Str("id", id).Msg("spawner.refund.trigger_failed")
	}
}
