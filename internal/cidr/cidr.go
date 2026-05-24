// Package cidr computes deterministic CIDR allocations for GKE subnets.
//
// The IPAM scheme allocates four coordinated CIDR ranges per GKE location
// (slot). Each slot index maps to exactly one address in each range:
//
//   - Primary (ip_cidr_range):          10.60.0.0/20,  increment by /20
//   - Pods (pod_ip_cidr_range):         10.0.0.0/15,   increment by /15
//   - Services (services_ip_cidr_range): 10.61.224.0/20, increment by /20
//   - Master (master_ipv4_cidr_block):  10.63.192.0/28, increment by /28
package cidr

import (
	"fmt"
	"net"
	"sort"

	"github.com/osinfra-io/pt-techne-mcp-server/internal/spec"
)

// Slot is one complete CIDR allocation across all four ranges.
type Slot struct {
	Index               int    `json:"slot"`
	IPCidrRange         string `json:"ip_cidr_range"`
	PodIPCidrRange      string `json:"pod_ip_cidr_range"`
	ServicesIPCidrRange string `json:"services_ip_cidr_range"`
	MasterIPv4CidrBlock string `json:"master_ipv4_cidr_block"`
}

// range defines one of the four CIDR progressions.
type cidrRange struct {
	baseIP uint32
	prefix int
}

var (
	primaryRange  = cidrRange{baseIP: ipToUint32(net.IPv4(10, 60, 0, 0)), prefix: 20}
	podRange      = cidrRange{baseIP: ipToUint32(net.IPv4(10, 0, 0, 0)), prefix: 15}
	servicesRange = cidrRange{baseIP: ipToUint32(net.IPv4(10, 61, 224, 0)), prefix: 20}
	masterRange   = cidrRange{baseIP: ipToUint32(net.IPv4(10, 63, 192, 0)), prefix: 28}
)

// SlotFor computes the full CIDR allocation for the given slot index.
func SlotFor(index int) Slot {
	return Slot{
		Index:               index,
		IPCidrRange:         cidrAt(primaryRange, index),
		PodIPCidrRange:      cidrAt(podRange, index),
		ServicesIPCidrRange: cidrAt(servicesRange, index),
		MasterIPv4CidrBlock: cidrAt(masterRange, index),
	}
}

// SlotFromSubnet returns the slot index for an existing subnet by
// reverse-mapping its primary CIDR. Returns -1 if the CIDR does not
// align with the IPAM progression.
func SlotFromSubnet(s spec.Subnet) int {
	return slotIndex(primaryRange, s.IPCidrRange)
}

// NextAvailable returns the next count unallocated slots given the set
// of existing subnets. It finds gaps in the allocation sequence and
// returns the lowest available slot indices.
func NextAvailable(existing []spec.Subnet, count int) []Slot {
	used := make(map[int]bool)
	for _, s := range existing {
		if idx := slotIndex(primaryRange, s.IPCidrRange); idx >= 0 {
			used[idx] = true
		}
	}

	slots := make([]Slot, 0, count)
	for i := 0; len(slots) < count; i++ {
		if !used[i] {
			slots = append(slots, SlotFor(i))
		}
	}
	return slots
}

// CollectSubnets extracts all Subnet values from a set of parsed teams.
func CollectSubnets(teams []*spec.Team) []spec.Subnet {
	var out []spec.Subnet
	for _, t := range teams {
		if t.PlatformManagedProject == nil || t.PlatformManagedProject.KubernetesEngine == nil {
			continue
		}
		for _, loc := range t.PlatformManagedProject.KubernetesEngine.Locations {
			out = append(out, loc.Subnet)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].IPCidrRange < out[j].IPCidrRange
	})
	return out
}

// cidrAt computes the CIDR string at a given slot index for a range.
func cidrAt(r cidrRange, index int) string {
	step := uint32(1) << (32 - r.prefix)
	ip := r.baseIP + uint32(index)*step
	return fmt.Sprintf("%s/%d", uint32ToIP(ip).String(), r.prefix)
}

// slotIndex reverse-maps a CIDR string to its slot index in the given
// range. Returns -1 if the CIDR does not parse or does not align.
func slotIndex(r cidrRange, cidr string) int {
	ip, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return -1
	}
	ones, _ := network.Mask.Size()
	if ones != r.prefix {
		return -1
	}
	addr := ipToUint32(ip.To4())
	if addr < r.baseIP {
		return -1
	}
	step := uint32(1) << (32 - r.prefix)
	offset := addr - r.baseIP
	if offset%step != 0 {
		return -1
	}
	return int(offset / step)
}

func ipToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func uint32ToIP(n uint32) net.IP {
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}
