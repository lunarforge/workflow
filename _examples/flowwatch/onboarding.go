package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/memrecordstore"
	"github.com/lunarforge/workflow/adapters/memrolescheduler"
	"github.com/lunarforge/workflow/adapters/memstreamer"
)

// OnboardingStatus represents states in the user onboarding workflow.
type OnboardingStatus int

const (
	OnboardingStatusUnknown        OnboardingStatus = 0
	OnboardingStatusCreated        OnboardingStatus = 1
	OnboardingStatusEmailVerified  OnboardingStatus = 2
	OnboardingStatusProfileSetup   OnboardingStatus = 3
	OnboardingStatusKYCPending     OnboardingStatus = 4
	OnboardingStatusKYCApproved    OnboardingStatus = 5
	OnboardingStatusKYCRejected    OnboardingStatus = 6
	OnboardingStatusWelcomeSent    OnboardingStatus = 7
	OnboardingStatusCompleted      OnboardingStatus = 8
	OnboardingStatusAbandoned      OnboardingStatus = 9
)

func (s OnboardingStatus) String() string {
	switch s {
	case OnboardingStatusCreated:
		return "Created"
	case OnboardingStatusEmailVerified:
		return "EmailVerified"
	case OnboardingStatusProfileSetup:
		return "ProfileSetup"
	case OnboardingStatusKYCPending:
		return "KYCPending"
	case OnboardingStatusKYCApproved:
		return "KYCApproved"
	case OnboardingStatusKYCRejected:
		return "KYCRejected"
	case OnboardingStatusWelcomeSent:
		return "WelcomeSent"
	case OnboardingStatusCompleted:
		return "Completed"
	case OnboardingStatusAbandoned:
		return "Abandoned"
	default:
		return "Unknown"
	}
}

// UserOnboarding is the business entity for the onboarding workflow.
type UserOnboarding struct {
	UserID         string    `json:"user_id"`
	Email          string    `json:"email"`
	DisplayName    string    `json:"display_name,omitempty"`
	Country        string    `json:"country"`
	KYCDocumentURL string    `json:"kyc_document_url,omitempty"`
	KYCScore       float64   `json:"kyc_score,omitempty"`
	WelcomeEmailID string    `json:"welcome_email_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func buildOnboardingWorkflow(
	streamer *memstreamer.StreamConstructor,
	recordStore *memrecordstore.Store,
	stepStore workflow.StepStore,
) *workflow.Workflow[UserOnboarding, OnboardingStatus] {
	b := workflow.NewBuilder[UserOnboarding, OnboardingStatus]("user-onboarding")

	b.AddStep(OnboardingStatusCreated, verifyEmail, OnboardingStatusEmailVerified, OnboardingStatusAbandoned)
	b.AddStep(OnboardingStatusEmailVerified, setupProfile, OnboardingStatusProfileSetup)
	b.AddStep(OnboardingStatusProfileSetup, submitKYC, OnboardingStatusKYCPending)
	b.AddStep(OnboardingStatusKYCPending, reviewKYC, OnboardingStatusKYCApproved, OnboardingStatusKYCRejected)
	b.AddStep(OnboardingStatusKYCApproved, sendWelcome, OnboardingStatusWelcomeSent)
	b.AddStep(OnboardingStatusWelcomeSent, completeOnboarding, OnboardingStatusCompleted)
	b.AddStep(OnboardingStatusKYCRejected, handleKYCRejection, OnboardingStatusKYCPending, OnboardingStatusAbandoned)

	return b.Build(
		streamer,
		recordStore,
		memrolescheduler.New(),
		workflow.WithStepStore(stepStore),
	)
}

func verifyEmail(_ context.Context, r *workflow.Run[UserOnboarding, OnboardingStatus]) (OnboardingStatus, error) {
	time.Sleep(150 * time.Millisecond)
	if r.Object.Email == "" {
		return OnboardingStatusAbandoned, nil
	}
	return OnboardingStatusEmailVerified, nil
}

func setupProfile(_ context.Context, r *workflow.Run[UserOnboarding, OnboardingStatus]) (OnboardingStatus, error) {
	time.Sleep(100 * time.Millisecond)
	if r.Object.DisplayName == "" {
		r.Object.DisplayName = fmt.Sprintf("user_%s", r.Object.UserID)
	}
	return OnboardingStatusProfileSetup, nil
}

func submitKYC(_ context.Context, r *workflow.Run[UserOnboarding, OnboardingStatus]) (OnboardingStatus, error) {
	time.Sleep(200 * time.Millisecond)
	r.Object.KYCDocumentURL = fmt.Sprintf("https://docs.example.com/%s/id.pdf", r.Object.UserID)
	return OnboardingStatusKYCPending, nil
}

func reviewKYC(_ context.Context, r *workflow.Run[UserOnboarding, OnboardingStatus]) (OnboardingStatus, error) {
	time.Sleep(400 * time.Millisecond) // Simulate external review
	// Reject users from "blocked-country"
	if r.Object.Country == "blocked-country" {
		r.Object.KYCScore = 0.2
		return OnboardingStatusKYCRejected, nil
	}
	r.Object.KYCScore = 0.95
	return OnboardingStatusKYCApproved, nil
}

func sendWelcome(_ context.Context, r *workflow.Run[UserOnboarding, OnboardingStatus]) (OnboardingStatus, error) {
	time.Sleep(100 * time.Millisecond)
	r.Object.WelcomeEmailID = fmt.Sprintf("email_%s", r.Object.UserID)
	return OnboardingStatusWelcomeSent, nil
}

func completeOnboarding(_ context.Context, _ *workflow.Run[UserOnboarding, OnboardingStatus]) (OnboardingStatus, error) {
	time.Sleep(50 * time.Millisecond)
	return OnboardingStatusCompleted, nil
}

func handleKYCRejection(_ context.Context, r *workflow.Run[UserOnboarding, OnboardingStatus]) (OnboardingStatus, error) {
	time.Sleep(100 * time.Millisecond)
	// After rejection, if score is > 0 allow re-submission, otherwise abandon
	if r.Object.KYCScore > 0.1 {
		return OnboardingStatusKYCPending, nil
	}
	return OnboardingStatusAbandoned, nil
}

func triggerSampleOnboarding(ctx context.Context, wf *workflow.Workflow[UserOnboarding, OnboardingStatus]) {
	users := []UserOnboarding{
		{
			// Happy path: passes KYC and completes onboarding.
			UserID:    "user-001",
			Email:     "alice@example.com",
			Country:   "US",
			CreatedAt: time.Now(),
		},
		{
			// KYC rejection: country is blocked.
			UserID:    "user-002",
			Email:     "bob@example.com",
			Country:   "blocked-country",
			CreatedAt: time.Now(),
		},
		{
			// Abandoned: no email triggers early exit.
			UserID:    "user-003",
			Email:     "",
			Country:   "GB",
			CreatedAt: time.Now(),
		},
	}

	for _, u := range users {
		fmt.Printf("Triggering onboarding %s (%s, %s)\n", u.UserID, u.Email, u.Country)
		_, err := wf.Trigger(ctx, u.UserID, workflow.WithInitialValue[UserOnboarding, OnboardingStatus](&u))
		if err != nil {
			fmt.Fprintf(os.Stderr, "trigger error for %s: %v\n", u.UserID, err)
		}
	}
}
