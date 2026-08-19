// Package machineid derives a stable installation fingerprint from platform
// identifiers without persisting or exposing the underlying hardware values.
package machineid

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Identifier is one stable platform-provided identifier used to derive the
// machine ID. Values are normalized before hashing.
type Identifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Info describes the source selected for the current machine fingerprint.
// Value is the raw stable identifier when one is available and is nil for the
// host fallback, so callers do not accidentally log hardware identifiers.
type Info struct {
	Platform  string  `json:"platform"`
	Type      string  `json:"type"`
	Value     *string `json:"value"`
	MachineID string  `json:"machineId"`
	DisplayID string  `json:"displayId"`
}

var virtualAdapterPrefixes = []string{
	"vmware", "virtualbox", "vbox", "hyper-v", "tap", "tunnel", "vpn",
	"loopback", "bluetooth", "wireless pan", "wsl", "docker", "vethernet",
	"virtual", "pseudo", "teredo", "isatap", "6to4", "hamachi", "zerotier",
	"tailscale", "nordvpn", "expressvpn", "surfshark", "protonvpn",
}

var serialWhitespace = regexp.MustCompile(`\s+`)

// GenerateMachineID returns the current machine fingerprint. It intentionally
// always returns a value; if platform identifiers are unavailable, a stable
// host fallback is used just like the legacy implementation.
func GenerateMachineID() string {
	identifiers := GetStableIdentifiers()
	if len(identifiers) > 0 {
		return MachineIDFromIdentifiers(identifiers)
	}
	host, _ := os.Hostname()
	return MachineIDFromFallback(host, systemMemoryBytes(), cpuModel())
}

// Generate is a concise alias for GenerateMachineID.
func Generate() string { return GenerateMachineID() }

// GenerateMachineId preserves the legacy mixed-case spelling.
func GenerateMachineId() string { return GenerateMachineID() }

// MachineIDFromIdentifiers derives the 16-character uppercase SHA-256 prefix
// used by the Node implementation from an ordered identifier list.
func MachineIDFromIdentifiers(identifiers []Identifier) string {
	values := make([]string, 0, len(identifiers))
	for _, identifier := range identifiers {
		if value := normalizeIdentifier(identifier.Value); value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return ""
	}
	return hashPrefix(strings.Join(values, "|"))
}

// MachineIDFromFallback mirrors the legacy hostname + total memory + CPU
// model fallback. It is exported so callers and tests can use a deterministic
// source without probing the host.
func MachineIDFromFallback(hostname string, totalMemory uint64, cpu string) string {
	return hashPrefix(hostname + strconv.FormatUint(totalMemory, 10) + cpu)
}

// GetMachineIDDisplay returns the machine ID grouped as XXXX-XXXX-XXXX-XXXX.
func GetMachineIDDisplay() string { return Display(GenerateMachineID()) }

// GetMachineIdDisplay preserves the legacy mixed-case spelling.
func GetMachineIdDisplay() string { return GetMachineIDDisplay() }

// Display formats a raw machine ID in four-character groups.
func Display(machineID string) string {
	raw := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(machineID), "-", ""))
	if raw == "" {
		return raw
	}
	var groups []string
	for len(raw) > 0 {
		n := 4
		if len(raw) < n {
			n = len(raw)
		}
		groups = append(groups, raw[:n])
		raw = raw[n:]
	}
	return strings.Join(groups, "-")
}

// GetIdentifierInfo returns metadata compatible with the legacy helper.
func GetIdentifierInfo() Info {
	identifiers := GetStableIdentifiers()
	info := Info{Platform: runtime.GOOS, Type: "fallback", MachineID: GenerateMachineID()}
	if len(identifiers) > 0 {
		info.Type = identifiers[0].Type
		value := identifiers[0].Value
		info.Value = &value
	}
	info.DisplayID = Display(info.MachineID)
	return info
}

// StableIdentifiers returns platform identifiers in the same priority order
// as the legacy implementation.
func StableIdentifiers() []Identifier { return GetStableIdentifiers() }

// GetStableIdentifiers returns UUID/serial identifiers that are stable across
// process restarts. Hardware values never leave this package except through
// the returned value, so callers should treat them as sensitive.
func GetStableIdentifiers() []Identifier {
	switch runtime.GOOS {
	case "windows":
		return windowsIdentifiers()
	case "linux":
		return linuxIdentifiers()
	case "darwin":
		return darwinIdentifiers()
	default:
		return nil
	}
}

// GetStableMacAddress returns a non-virtual, non-local MAC address with an
// IPv4 interface, matching the legacy helper's filtering rules.
func GetStableMacAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	type candidate struct {
		name string
		mac  string
	}
	candidates := make([]candidate, 0, len(interfaces))
	for _, iface := range interfaces {
		if isVirtualAdapter(iface.Name) || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) != 6 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if !hasIPv4(iface) {
			continue
		}
		mac := strings.ToUpper(strings.ReplaceAll(iface.HardwareAddr.String(), ":", ""))
		if mac == "000000000000" || strings.HasPrefix(mac, "00") || strings.HasPrefix(mac, "02") {
			continue
		}
		candidates = append(candidates, candidate{name: iface.Name, mac: mac})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].name < candidates[j].name })
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].mac
}

// StableMACAddress is an alias for GetStableMacAddress.
func StableMACAddress() string { return GetStableMacAddress() }

func hasIPv4(iface net.Interface) bool {
	addresses, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, address := range addresses {
		ipNet, ok := address.(*net.IPNet)
		if ok && ipNet.IP.To4() != nil && !ipNet.IP.IsLoopback() {
			return true
		}
		if ip, _, err := net.ParseCIDR(address.String()); err == nil && ip.To4() != nil && !ip.IsLoopback() {
			return true
		}
	}
	return false
}

func isVirtualAdapter(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range virtualAdapterPrefixes {
		if strings.Contains(lower, prefix) {
			return true
		}
	}
	return false
}

func windowsIdentifiers() []Identifier {
	identifiers := make([]Identifier, 0, 3)
	if value := queryWMIC("csproduct", "UUID", 10); value != "" {
		identifiers = append(identifiers, Identifier{Type: "uuid", Value: value})
	}
	if value := queryWMIC("bios", "SerialNumber", 4); value != "" {
		identifiers = append(identifiers, Identifier{Type: "bios", Value: value})
	}
	if value := queryWMIC("baseboard", "SerialNumber", 4); value != "" {
		identifiers = append(identifiers, Identifier{Type: "baseboard", Value: value})
	}
	return identifiers
}

func queryWMIC(alias, field string, minimum int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "wmic", alias, "get", field).Output()
	if err != nil {
		return ""
	}
	lines := strings.Fields(string(output))
	for _, line := range lines {
		if strings.EqualFold(line, field) {
			continue
		}
		value := normalizeIdentifier(line)
		if len(value) >= minimum {
			return value
		}
	}
	return ""
}

func linuxIdentifiers() []Identifier {
	identifiers := make([]Identifier, 0, 2)
	if value := readIdentifierFile("/etc/machine-id", 10); value != "" {
		identifiers = append(identifiers, Identifier{Type: "machine-id", Value: value})
	}
	if value := readIdentifierFile("/sys/class/dmi/id/product_uuid", 10); value != "" {
		identifiers = append(identifiers, Identifier{Type: "dmi-uuid", Value: value})
	}
	return identifiers
}

func darwinIdentifiers() []Identifier {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return nil
	}
	match := regexp.MustCompile(`IOPlatformUUID\s*=\s*"([^"]+)"`).FindStringSubmatch(string(output))
	if len(match) < 2 {
		return nil
	}
	value := normalizeIdentifier(match[1])
	if len(value) < 10 {
		return nil
	}
	return []Identifier{{Type: "platform-uuid", Value: value}}
}

func readIdentifierFile(path string, minimum int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	value := normalizeIdentifier(string(data))
	if len(value) < minimum {
		return ""
	}
	return value
}

func normalizeIdentifier(value string) string {
	return strings.ToUpper(strings.ReplaceAll(serialWhitespace.ReplaceAllString(strings.TrimSpace(value), ""), "-", ""))
}

func hashPrefix(value string) string {
	sum := sha256.Sum256([]byte(value))
	return strings.ToUpper(hex.EncodeToString(sum[:])[:16])
}

func systemMemoryBytes() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				return kb * 1024
			}
		}
	}
	return 0
}

func cpuModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if key, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(key), "model name") {
				return strings.TrimSpace(value)
			}
		}
	}
	return runtime.GOARCH
}

// ValidateIdentifier is useful to callers that accept a machine ID from a
// license file or administrator input.
func ValidateIdentifier(value string) error {
	normalized := normalizeIdentifier(value)
	if normalized == "" {
		return errors.New("machine identifier is empty")
	}
	return nil
}
