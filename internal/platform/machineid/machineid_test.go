package machineid

import "testing"

func TestMachineIDFromIdentifiersMatchesHashContract(t *testing.T) {
	identifiers := []Identifier{{Type: "first", Value: "ab-c"}, {Type: "second", Value: " def "}}
	if got, want := MachineIDFromIdentifiers(identifiers), "470F0E01A37B5DC5"; got != want {
		t.Fatalf("MachineIDFromIdentifiers() = %q, want %q", got, want)
	}
}

func TestMachineIDFromFallbackAndDisplay(t *testing.T) {
	if got, want := MachineIDFromFallback("host", 123456, "cpu"), "AD3ED8A58710FA6A"; got != want {
		t.Fatalf("MachineIDFromFallback() = %q, want %q", got, want)
	}
	if got, want := Display("aabbccddeeff0011"), "AABB-CCDD-EEFF-0011"; got != want {
		t.Fatalf("Display() = %q, want %q", got, want)
	}
}

func TestGenerateMachineIDIsStableAndFormatted(t *testing.T) {
	first := GenerateMachineID()
	second := GenerateMachineID()
	if first == "" || first != second || len(first) != 16 {
		t.Fatalf("machine ID is not stable: %q / %q", first, second)
	}
	for _, character := range first {
		if !(character >= '0' && character <= '9') && !(character >= 'A' && character <= 'F') {
			t.Fatalf("machine ID contains non-hex character %q", character)
		}
	}
}
