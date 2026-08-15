package device

import (
	"context"
	"testing"
)

// injectSnapshot stores a snapshot on the managed device so callers observe its
// IMSI without replaying the full readSnapshot AT transcript.
func injectSnapshot(t *testing.T, manager *Manager, id string, snapshot *Snapshot) {
	t.Helper()
	state, err := manager.lookup(id)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	manager.setResult(id, state, snapshot, nil)
}

func TestCardMCCMNC(t *testing.T) {
	t.Parallel()
	if mcc, _ := CardMCCMNC("460001234567890"); mcc != "460" {
		t.Fatalf("CardMCCMNC mcc = %q, want 460", mcc)
	}
	if mcc, mnc := CardMCCMNCWithLength("454006395879502", 2); mcc != "454" || mnc != "00" {
		t.Fatalf("CardMCCMNCWithLength = (%q, %q), want (454, 00)", mcc, mnc)
	}
	for _, bad := range []string{"", "4600", "4600X1234"} {
		if mcc, _ := CardMCCMNC(bad); mcc != "" {
			t.Fatalf("CardMCCMNC(%q) mcc = %q, want empty", bad, mcc)
		}
	}
}

func networkEnableTranscript() *transcriptClient {
	return &transcriptClient{steps: []clientStep{
		{command: `AT+CGDCONT=1,"IPV4V6","internet"`, response: okResponse()},
		{command: "AT+CGATT=1", response: okResponse()},
		{command: "AT+CGACT=1,1", response: okResponse()},
	}}
}

// The card's home MCC no longer gates service: every region attaches, and so
// does a card whose region is not known yet.
func TestSetNetworkEnabledForCardRegion(t *testing.T) {
	for name, imsi := range map[string]string{
		"us":      "310260123456789",
		"cn":      "460001234567890",
		"cn_alt":  "461001234567890",
		"unknown": "",
	} {
		t.Run(name, func(t *testing.T) {
			client := networkEnableTranscript()
			manager, id := newStartedTestManager(t, client)
			if imsi != "" {
				injectSnapshot(t, manager, id, &Snapshot{DeviceID: id, IMSI: imsi})
			}
			result, err := manager.SetNetwork(context.Background(), id, NetworkRequest{
				Enabled: true, APN: "internet", IPVersion: "IPV4V6",
			})
			if err != nil {
				t.Fatalf("enable network: %v", err)
			}
			if !result.Enabled {
				t.Fatalf("enable result = %#v", result)
			}
			client.assertDone(t)
		})
	}
}
