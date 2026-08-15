package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vocat/internal/device"
	"vocat/internal/modem"
	"vocat/internal/store"
)

// fakeDeviceController is a stub DeviceController that resolves one discovered
// device carrying a fixed snapshot, enough to drive the region guards. The scan,
// USSD, and USB-net results are configurable for the feature endpoint tests.
type fakeDeviceController struct {
	entry      device.Device
	atResponse modem.Response
	atErr      error
	atHandler  func(string) (modem.Response, error)
	scanResult device.OperatorScanResult
	scanErr    error
	ussdResult device.USSDResult
	ussdErr    error
	usbNetMode device.USBNetMode
	usbNetErr  error
}

func (f fakeDeviceController) Discover(context.Context) ([]device.Device, error) {
	return []device.Device{f.entry}, nil
}
func (f fakeDeviceController) List() []device.Device { return []device.Device{f.entry} }
func (f fakeDeviceController) Get(id string) (device.Device, error) {
	if id == f.entry.ID {
		return f.entry, nil
	}
	return device.Device{}, device.ErrNotFound
}
func (f fakeDeviceController) Refresh(context.Context, string) (device.Snapshot, error) {
	return device.Snapshot{}, nil
}

func (f fakeDeviceController) ExecuteAT(_ context.Context, _ string, command string) (modem.Response, error) {
	if f.atHandler != nil {
		return f.atHandler(command)
	}
	return f.atResponse, f.atErr
}
func (f fakeDeviceController) Reboot(context.Context, string) error { return nil }
func (f fakeDeviceController) USSD(context.Context, string, string) (device.USSDResult, error) {
	return f.ussdResult, f.ussdErr
}
func (f fakeDeviceController) ContinueUSSD(context.Context, string, string) (device.USSDResult, error) {
	return f.ussdResult, f.ussdErr
}
func (f fakeDeviceController) CancelUSSD(context.Context, string) error { return f.ussdErr }
func (f fakeDeviceController) SetFlight(context.Context, string, bool) (device.FlightResult, error) {
	return device.FlightResult{}, nil
}
func (f fakeDeviceController) SetNetwork(context.Context, string, device.NetworkRequest) (device.NetworkResult, error) {
	return device.NetworkResult{}, nil
}
func (f fakeDeviceController) USBNetMode(context.Context, string) (device.USBNetMode, error) {
	return device.USBNetMode{}, nil
}
func (f fakeDeviceController) SetUSBNetMode(context.Context, string, int) (device.USBNetMode, error) {
	return device.USBNetMode{}, nil
}
func (f fakeDeviceController) SetUSBNetModeByPort(context.Context, string, int) (device.USBNetMode, error) {
	return f.usbNetMode, f.usbNetErr
}
func (f fakeDeviceController) OperatorSelection(context.Context, string) (device.OperatorSelection, error) {
	return device.OperatorSelection{}, nil
}
func (f fakeDeviceController) SetOperatorSelection(context.Context, string, bool, string, *int) (device.OperatorSelection, error) {
	return device.OperatorSelection{}, nil
}

func (f fakeDeviceController) ReRegisterOperator(context.Context, string) (device.OperatorSelection, error) {
	return device.OperatorSelection{}, nil
}
func (f fakeDeviceController) ScanOperators(context.Context, string) (device.OperatorScanResult, error) {
	return f.scanResult, f.scanErr
}
func (f fakeDeviceController) SendSMS(context.Context, string, string, string) (device.SMSSendResult, error) {
	return device.SMSSendResult{}, nil
}
func (f fakeDeviceController) ListSMS(context.Context, string) ([]device.SMSMessage, error) {
	return nil, nil
}
func (f fakeDeviceController) ReadSMS(context.Context, string, int) (device.SMSMessage, error) {
	return device.SMSMessage{}, nil
}
func (f fakeDeviceController) DeleteSMS(context.Context, string, int) error { return nil }
func (f fakeDeviceController) ESIMListProfiles(context.Context, string) (device.EsimInfo, error) {
	return device.EsimInfo{}, nil
}
func (f fakeDeviceController) ESIMInventory(context.Context, string) ([]device.EsimInventoryEntry, error) {
	return []device.EsimInventoryEntry{}, nil
}
func (f fakeDeviceController) ESIMSwitchProfile(context.Context, string, string, string) error {
	return nil
}
func (f fakeDeviceController) ESIMDisableProfile(context.Context, string, string, string) error {
	return nil
}
func (f fakeDeviceController) ESIMRenameProfile(context.Context, string, string, string, string) error {
	return nil
}
func (f fakeDeviceController) ESIMDownloadProfile(context.Context, string, device.EsimDownloadParams, func(device.EsimProgress)) (*device.EsimDownloadResult, error) {
	return nil, errors.New("not implemented in test fake")
}
func (f fakeDeviceController) ESIMDeleteProfile(context.Context, string, string, string) (*device.EsimDeleteResult, error) {
	return &device.EsimDeleteResult{}, nil
}
func (f fakeDeviceController) ESIMChipInfo(context.Context, string) (*device.EsimChipInfo, error) {
	return nil, errors.New("not implemented in test fake")
}

func regionTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func regionTestServer(t *testing.T, imsi string) *Server {
	t.Helper()
	database, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return &Server{
		store:               database,
		logger:              regionTestLogger(),
		maxRequestBodyBytes: 4096,
		devices: fakeDeviceController{entry: device.Device{
			ID:         "dev1",
			Discovered: true,
			Snapshot:   &device.Snapshot{DeviceID: "dev1", IMSI: imsi},
		}},
		vowifi: &fakeVoWiFiController{},
	}
}

func TestModemSummaryCardRegionFields(t *testing.T) {
	t.Parallel()
	// The card's home region is reported for display only; nothing in the
	// summary marks a card as barred from service.
	for imsi, want := range map[string][2]string{
		"460001234567890": {"460", "中国"},
		"310260123456789": {"310", "美国"},
	} {
		summary := modemSummary(&device.Snapshot{IMSI: imsi}, "", "")
		if summary["card_mcc"] != want[0] || summary["card_country"] != want[1] {
			t.Fatalf("IMSI %s: card_mcc=%v card_country=%v, want %v", imsi, summary["card_mcc"], summary["card_country"], want)
		}
		for _, key := range []string{"service_blocked", "blocked_reason"} {
			if _, present := summary[key]; present {
				t.Fatalf("IMSI %s: summary still reports %q", imsi, key)
			}
		}
	}

	empty := modemSummary(nil, "", "")
	if empty["card_mcc"] != "" {
		t.Fatalf("nil snapshot card_mcc = %v, want empty", empty["card_mcc"])
	}
}

func TestCountryNameForMCC(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"460": "中国",
		"461": "中国",
		"454": "中国香港",
		"466": "中国台湾",
		"310": "美国",
		"":    "",
		"999": "",
	}
	for mcc, want := range cases {
		if got := countryNameForMCC(mcc); got != want {
			t.Errorf("countryNameForMCC(%q) = %q, want %q", mcc, got, want)
		}
	}
}

// VoWiFi is available to every card region, including the MCCs that earlier
// versions refused to serve.
func TestHandleVoWiFiEnabledIsNotRegionGated(t *testing.T) {
	for _, imsi := range []string{"460001234567890", "461001234567890", "310260123456789"} {
		server := regionTestServer(t, imsi)
		// Seed the device so the UpsertDevice that follows the guards succeeds.
		if err := server.store.UpsertDevice(context.Background(), store.Device{ID: "dev1", Name: "EC20"}); err != nil {
			t.Fatalf("seed device: %v", err)
		}
		request := httptest.NewRequest(http.MethodPatch, "/devices/dev1/vowifi", strings.NewReader(`{"enabled":true}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()

		if handled := server.handleVoWiFiEnabled(recorder, request, store.Device{ID: "dev1"}, true); !handled {
			t.Fatalf("IMSI %s: handler did not claim the request", imsi)
		}
		if recorder.Code == http.StatusForbidden {
			t.Fatalf("IMSI %s: must not be blocked, got 403: %s", imsi, recorder.Body.String())
		}
	}
}
