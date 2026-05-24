package tools_test

import (
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/cidr"
	"github.com/osinfra-io/pt-techne-mcp-server/internal/tools"
)

func TestNextAvailableCidrs_Goldens(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)

	res := h.call("next_available_cidrs", map[string]any{"count": 2})
	var out tools.NextAvailableCidrsOutput
	decodeStruct(t, res, &out)

	if len(out.Slots) != 2 {
		t.Fatalf("expected 2 slots, got %d: %+v", len(out.Slots), out.Slots)
	}
	// Golden data has 8 locations (slots 0-7), so next should be 8 and 9.
	if out.Slots[0].Index != 8 {
		t.Errorf("first slot index = %d, want 8", out.Slots[0].Index)
	}
	if out.Slots[1].Index != 9 {
		t.Errorf("second slot index = %d, want 9", out.Slots[1].Index)
	}
	// Verify the CIDRs match SlotFor computation.
	want8 := cidr.SlotFor(8)
	if out.Slots[0].IPCidrRange != want8.IPCidrRange {
		t.Errorf("slot 8 primary = %s, want %s", out.Slots[0].IPCidrRange, want8.IPCidrRange)
	}
	if out.Slots[0].PodIPCidrRange != want8.PodIPCidrRange {
		t.Errorf("slot 8 pod = %s, want %s", out.Slots[0].PodIPCidrRange, want8.PodIPCidrRange)
	}
}

func TestNextAvailableCidrs_CountOne(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)

	res := h.call("next_available_cidrs", map[string]any{"count": 1})
	var out tools.NextAvailableCidrsOutput
	decodeStruct(t, res, &out)

	if len(out.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(out.Slots))
	}
	if out.Slots[0].Index != 8 {
		t.Errorf("slot index = %d, want 8", out.Slots[0].Index)
	}
}

func TestNextAvailableCidrs_InvalidCount(t *testing.T) {
	f := seedFakeFromGoldens(t)
	h := newReadHarness(t, f)

	// count = 0
	res := h.call("next_available_cidrs", map[string]any{"count": 0})
	body := decodeError(t, res)
	if body["code"] != "invalid_input" {
		t.Fatalf("expected invalid_input, got %+v", body)
	}

	// count too large
	res = h.call("next_available_cidrs", map[string]any{"count": 999})
	body = decodeError(t, res)
	if body["code"] != "invalid_input" {
		t.Fatalf("expected invalid_input for large count, got %+v", body)
	}
}
