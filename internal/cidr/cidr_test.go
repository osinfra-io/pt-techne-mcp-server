package cidr

import (
	"testing"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

func TestSlotFor_KnownSlots(t *testing.T) {
	// Verified against golden data from pt-pneuma.tfvars and pt-kryptos.tfvars.
	tests := []struct {
		slot    int
		primary string
		pod     string
		svc     string
		master  string
	}{
		{0, "10.60.0.0/20", "10.0.0.0/15", "10.61.224.0/20", "10.63.192.0/28"},
		{1, "10.60.16.0/20", "10.2.0.0/15", "10.61.240.0/20", "10.63.192.16/28"},
		{2, "10.60.32.0/20", "10.4.0.0/15", "10.62.0.0/20", "10.63.192.32/28"},
		{3, "10.60.48.0/20", "10.6.0.0/15", "10.62.16.0/20", "10.63.192.48/28"},
		{4, "10.60.64.0/20", "10.8.0.0/15", "10.62.32.0/20", "10.63.192.64/28"},
		{5, "10.60.80.0/20", "10.10.0.0/15", "10.62.48.0/20", "10.63.192.80/28"},
		{6, "10.60.96.0/20", "10.12.0.0/15", "10.62.64.0/20", "10.63.192.96/28"},
		{7, "10.60.112.0/20", "10.14.0.0/15", "10.62.80.0/20", "10.63.192.112/28"},
	}
	for _, tt := range tests {
		s := SlotFor(tt.slot)
		if s.IPCidrRange != tt.primary {
			t.Errorf("slot %d primary: got %s, want %s", tt.slot, s.IPCidrRange, tt.primary)
		}
		if s.PodIPCidrRange != tt.pod {
			t.Errorf("slot %d pod: got %s, want %s", tt.slot, s.PodIPCidrRange, tt.pod)
		}
		if s.ServicesIPCidrRange != tt.svc {
			t.Errorf("slot %d services: got %s, want %s", tt.slot, s.ServicesIPCidrRange, tt.svc)
		}
		if s.MasterIPv4CidrBlock != tt.master {
			t.Errorf("slot %d master: got %s, want %s", tt.slot, s.MasterIPv4CidrBlock, tt.master)
		}
	}
}

func TestSlotFromSubnet(t *testing.T) {
	tests := []struct {
		subnet spec.Subnet
		want   int
	}{
		{spec.Subnet{IPCidrRange: "10.60.0.0/20"}, 0},
		{spec.Subnet{IPCidrRange: "10.60.16.0/20"}, 1},
		{spec.Subnet{IPCidrRange: "10.60.112.0/20"}, 7},
		{spec.Subnet{IPCidrRange: "invalid"}, -1},
		{spec.Subnet{IPCidrRange: "10.60.0.0/24"}, -1}, // wrong prefix
		{spec.Subnet{IPCidrRange: "10.60.1.0/20"}, -1}, // misaligned
	}
	for _, tt := range tests {
		got := SlotFromSubnet(tt.subnet)
		if got != tt.want {
			t.Errorf("SlotFromSubnet(%q) = %d, want %d", tt.subnet.IPCidrRange, got, tt.want)
		}
	}
}

func TestNextAvailable_NoExisting(t *testing.T) {
	slots := NextAvailable(nil, 3)
	if len(slots) != 3 {
		t.Fatalf("got %d slots, want 3", len(slots))
	}
	for i, s := range slots {
		if s.Index != i {
			t.Errorf("slot[%d].Index = %d, want %d", i, s.Index, i)
		}
	}
}

func TestNextAvailable_WithGaps(t *testing.T) {
	// Slots 0, 1, 3 are allocated — next available should be 2, 4, 5.
	existing := []spec.Subnet{
		{IPCidrRange: "10.60.0.0/20"},  // slot 0
		{IPCidrRange: "10.60.16.0/20"}, // slot 1
		{IPCidrRange: "10.60.48.0/20"}, // slot 3
	}
	slots := NextAvailable(existing, 3)
	if len(slots) != 3 {
		t.Fatalf("got %d slots, want 3", len(slots))
	}
	wantIndices := []int{2, 4, 5}
	for i, s := range slots {
		if s.Index != wantIndices[i] {
			t.Errorf("slot[%d].Index = %d, want %d", i, s.Index, wantIndices[i])
		}
	}
}

func TestNextAvailable_AllContiguous(t *testing.T) {
	// Slots 0-7 allocated, next should be 8.
	existing := make([]spec.Subnet, 8)
	for i := range existing {
		existing[i] = spec.Subnet{IPCidrRange: SlotFor(i).IPCidrRange}
	}
	slots := NextAvailable(existing, 1)
	if len(slots) != 1 {
		t.Fatalf("got %d slots, want 1", len(slots))
	}
	if slots[0].Index != 8 {
		t.Errorf("got index %d, want 8", slots[0].Index)
	}
}

func TestCollectSubnets(t *testing.T) {
	teams := []*spec.Team{
		{TeamKey: "pt-a"}, // no GKE
		{
			TeamKey: "pt-b",
			PlatformManagedProject: &spec.PlatformManagedProject{
				KubernetesEngine: &spec.KubernetesEngine{
					Locations: map[string]spec.GKELocation{
						"us-east1-b": {Subnet: spec.Subnet{IPCidrRange: "10.60.16.0/20"}},
						"us-east1-c": {Subnet: spec.Subnet{IPCidrRange: "10.60.0.0/20"}},
					},
				},
			},
		},
	}
	subs := CollectSubnets(teams)
	if len(subs) != 2 {
		t.Fatalf("got %d subnets, want 2", len(subs))
	}
	// Sorted by ip_cidr_range.
	if subs[0].IPCidrRange != "10.60.0.0/20" {
		t.Errorf("first subnet = %s, want 10.60.0.0/20", subs[0].IPCidrRange)
	}
}
