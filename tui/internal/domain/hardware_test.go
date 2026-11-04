package domain_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Andr3xDev/nuydeck/tui/internal/domain"
)

// Records are shaped like real `lspci -vmmnn` output (tab after the tag, blank line between records).
const (
	lspciHostBridge = "Slot:\t00:00.0\nClass:\tHost bridge [0600]\nVendor:\tIntel Corporation [8086]\n" +
		"Device:\t12th Gen Core Processor Host Bridge/DRAM Registers [4629]\nSVendor:\tAcer Incorporated [ALI] [1025]\n" +
		"SDevice:\tDevice [159c]\nRev:\t02\nProgIf:\t00\nIOMMUGroup:\t1\n\n"
	lspciIntelGPU = "Slot:\t00:02.0\nClass:\tVGA compatible controller [0300]\nVendor:\tIntel Corporation [8086]\n" +
		"Device:\tAlder Lake-P GT1 [UHD Graphics] [46a3]\nSVendor:\tAcer Incorporated [ALI] [1025]\n" +
		"SDevice:\tDevice [159c]\nRev:\t0c\nProgIf:\t00\nIOMMUGroup:\t0\n\n"
	lspciNvidiaGPU = "Slot:\t01:00.0\nClass:\t3D controller [0302]\nVendor:\tNVIDIA Corporation [10de]\n" +
		"Device:\tAD107M [GeForce RTX 4050 Max-Q / Mobile] [28a1]\nSVendor:\tAcer Incorporated [ALI] [1025]\n" +
		"SDevice:\tDevice [159c]\nRev:\ta1\nIOMMUGroup:\t12\n\n"
	lspciAMDGPU = "Slot:\t06:00.0\nClass:\tDisplay controller [0380]\nVendor:\tAdvanced Micro Devices, Inc. [AMD/ATI] [1002]\n" +
		"Device:\tPhoenix1 [1681]\nSVendor:\tAdvanced Micro Devices, Inc. [AMD/ATI] [1002]\n" +
		"SDevice:\tDevice [1681]\nRev:\tc4\nUnknownTag:\twhatever\nIOMMUGroup:\t5\n\n"
	lspciGarbage = "this is not lspci output\n\n\x00\x01 [zz]\nClass: no tab [0300]\n"
)

var testDrivers = []domain.Driver{
	{ID: "intel", VendorID: "8086"},
	{ID: "nvidia", VendorID: "10de"},
	{ID: "amd", VendorID: "1002"},
}

func TestLspciParse(t *testing.T) {
	got := domain.ParseLspci(lspciHostBridge + lspciIntelGPU + lspciNvidiaGPU + lspciAMDGPU)
	want := []domain.PCIDevice{
		{Slot: "00:00.0", ClassID: "0600", VendorID: "8086"},
		{Slot: "00:02.0", ClassID: "0300", VendorID: "8086"},
		{Slot: "01:00.0", ClassID: "0302", VendorID: "10de"},
		{Slot: "06:00.0", ClassID: "0380", VendorID: "1002"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ParseLspci() = %+v, want %+v", got, want)
	}
}

func TestLspciParseGarbageYieldsNoUsableDevice(t *testing.T) {
	if devices := domain.ParseLspci(lspciGarbage); len(devices) != 0 {
		t.Errorf("garbage produced devices: %+v", devices)
	}
}

func TestProposeDrivers(t *testing.T) {
	allIDs := []string{"amd", "intel", "nvidia"}
	tests := []struct {
		name         string
		lspci        string
		want         []string
		wantDetected bool
	}{
		{"intel only", lspciHostBridge + lspciIntelGPU, []string{"intel"}, true},
		{"nvidia only", lspciNvidiaGPU, []string{"nvidia"}, true},
		{"amd only", lspciAMDGPU, []string{"amd"}, true},
		{"intel and nvidia", lspciIntelGPU + lspciNvidiaGPU, []string{"intel", "nvidia"}, true},
		{"amd and nvidia", lspciAMDGPU + lspciNvidiaGPU, []string{"amd", "nvidia"}, true},
		{"two nvidia cards yield one id", lspciNvidiaGPU + strings.Replace(lspciNvidiaGPU, "01:00.0", "02:00.0", 1), []string{"nvidia"}, true},
		{"non-GPU device with a known vendor falls back", lspciHostBridge, allIDs, false},
		{"garbage falls back", lspciGarbage, allIDs, false},
		{"empty falls back", "", allIDs, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ids, detected := domain.ProposeDrivers(domain.ParseLspci(tc.lspci), testDrivers)
			slices.Sort(ids)
			if detected != tc.wantDetected || !slices.Equal(ids, tc.want) {
				t.Fatalf("ProposeDrivers() = %v, %v; want %v, %v", ids, detected, tc.want, tc.wantDetected)
			}
		})
	}
}

func TestProposeDriversMatchesVendorCaseInsensitively(t *testing.T) {
	ids, detected := domain.ProposeDrivers(domain.ParseLspci(lspciNvidiaGPU), []domain.Driver{{ID: "nvidia", VendorID: "10DE"}})
	if !detected || !slices.Equal(ids, []string{"nvidia"}) {
		t.Fatalf("ProposeDrivers() = %v, %v", ids, detected)
	}
}

func TestGPUEnvFileName(t *testing.T) {
	tests := []struct {
		ids  []string
		want string
	}{
		{[]string{"amd"}, "gpu_env_amd.lua"},
		{[]string{"intel"}, "gpu_env_intel.lua"},
		{[]string{"nvidia"}, "gpu_env_nvidia.lua"},
		{[]string{"nvidia", "intel"}, "gpu_env_intel_nvidia.lua"},
		{[]string{"nvidia", "amd"}, "gpu_env_amd_nvidia.lua"},
		{[]string{"intel", "amd"}, "gpu_env_amd_intel.lua"},
		{[]string{"nvidia", "intel", "amd"}, "gpu_env_amd_intel_nvidia.lua"},
	}
	if got := domain.GPUEnvFileName(nil); got != "" {
		t.Errorf("GPUEnvFileName(nil) = %q, want empty", got)
	}
	seen := map[string]bool{}
	for _, tc := range tests {
		got := domain.GPUEnvFileName(tc.ids)
		if got != tc.want {
			t.Errorf("GPUEnvFileName(%v) = %q, want %q", tc.ids, got, tc.want)
		}
		seen[got] = true
	}
	if len(seen) != 7 {
		t.Errorf("distinct file names = %d, want 7", len(seen))
	}
}

func TestGPUEnvFileNameDoesNotReorderCallerSlice(t *testing.T) {
	ids := []string{"nvidia", "amd"}
	domain.GPUEnvFileName(ids)
	if strings.Join(ids, ",") != "nvidia,amd" {
		t.Fatalf("input mutated: %v", ids)
	}
}
