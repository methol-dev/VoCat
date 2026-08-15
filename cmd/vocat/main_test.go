package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"vocat/internal/store"
)

func regionTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newRegionTestStore(t *testing.T) *store.Store {
	t.Helper()
	database, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// Databases written by earlier versions can still hold the auto policies that
// pinned a barred SIM to airplane mode. Startup must drop them so the card is
// governed by the normal default policy again.
func TestClearLegacyRegionBlockPoliciesRemovesAutoBlocks(t *testing.T) {
	ctx := context.Background()
	database := newRegionTestStore(t)

	if err := database.UpsertCardPolicy(ctx, store.CardPolicy{
		ICCID:           "89860012345678901234",
		AirplaneEnabled: true,
		IPVersion:       "IPV4V6",
		Source:          cardPolicySourceRegionBlock,
	}); err != nil {
		t.Fatalf("seed legacy block policy: %v", err)
	}

	clearLegacyRegionBlockPolicies(ctx, regionTestLogger(), database)

	if _, err := database.CardPolicy(ctx, "89860012345678901234"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected the legacy block policy to be cleared, got err=%v", err)
	}
}

// Only the auto-written policies are cleared: a policy the user configured must
// survive, whatever the card's home region is.
func TestClearLegacyRegionBlockPoliciesKeepsUserPolicies(t *testing.T) {
	ctx := context.Background()
	database := newRegionTestStore(t)

	userPolicy := store.CardPolicy{
		ICCID:           "89860012345678901234",
		VoWiFiEnabled:   true,
		AirplaneEnabled: true,
		IPVersion:       "IPV4V6",
		Source:          "default",
	}
	if err := database.UpsertCardPolicy(ctx, userPolicy); err != nil {
		t.Fatalf("seed user policy: %v", err)
	}

	clearLegacyRegionBlockPolicies(ctx, regionTestLogger(), database)

	stored, err := database.CardPolicy(ctx, userPolicy.ICCID)
	if err != nil {
		t.Fatalf("CardPolicy: %v", err)
	}
	if stored.Source != "default" || !stored.VoWiFiEnabled {
		t.Fatalf("user policy = %#v, want it preserved", stored)
	}
}

func TestClearLegacyRegionBlockPoliciesWithNoPolicies(t *testing.T) {
	ctx := context.Background()
	database := newRegionTestStore(t)

	clearLegacyRegionBlockPolicies(ctx, regionTestLogger(), database)

	policies, err := database.ListCardPolicies(ctx)
	if err != nil {
		t.Fatalf("ListCardPolicies: %v", err)
	}
	if len(policies) != 0 {
		t.Fatalf("expected no card policies, got %d", len(policies))
	}
}
