package winattr

import "strings"

var wellKnownPrincipal = map[string]string{
	"BA": "BUILTIN\\Administrators",
	"BU": "BUILTIN\\Users",
	"BG": "BUILTIN\\Guests",
	"SY": "SYSTEM",
	"WD": "Everyone",
	"AN": "ANONYMOUS",
	"NU": "NETWORK",
	"AO": "Account operators",
	"PO": "Print operators",
	"SO": "Server operators",
	"PA": "Group Policy administrators",
	"CA": "Certificate administrators",
	"SA": "Schema administrators",
	"EA": "Enterprise administrators",
	"DA": "Domain administrators",
	"IW": "INTERACTIVE",
	"CO": "CREATOR OWNER",
	"CF": "CREATOR GROUP",
}

// OwnerLabel returns a human-readable owner from SDDL (O: component).
func OwnerLabel(sddl string) string {
	sid := ownerSIDFromSDDL(sddl)
	if sid == "" {
		return ""
	}
	if name, ok := wellKnownPrincipal[sid]; ok {
		return name
	}
	if resolved := lookupAccountName(sid); resolved != "" {
		return resolved
	}
	return sid
}

// AttributesLabel returns a short comma-separated list of file attribute flags.
func AttributesLabel(attrs uint32) string {
	if attrs == 0 {
		return ""
	}
	var parts []string
	if attrs&0x1 != 0 {
		parts = append(parts, "R")
	}
	if attrs&0x2 != 0 {
		parts = append(parts, "H")
	}
	if attrs&0x4 != 0 {
		parts = append(parts, "S")
	}
	if attrs&0x10 != 0 {
		parts = append(parts, "D")
	}
	if attrs&0x20 != 0 {
		parts = append(parts, "A")
	}
	if attrs&0x100 != 0 {
		parts = append(parts, "T")
	}
	if attrs&0x400 != 0 {
		parts = append(parts, "C")
	}
	if attrs&0x800 != 0 {
		parts = append(parts, "E")
	}
	if attrs&0x1000 != 0 {
		parts = append(parts, "O")
	}
	if attrs&0x2000 != 0 {
		parts = append(parts, "I")
	}
	if attrs&0x4000 != 0 {
		parts = append(parts, "N")
	}
	return strings.Join(parts, " ")
}

func ownerSIDFromSDDL(sddl string) string {
	sddl = strings.TrimSpace(sddl)
	if sddl == "" {
		return ""
	}
	idx := strings.Index(sddl, "O:")
	if idx < 0 {
		return ""
	}
	rest := sddl[idx+2:]
	end := len(rest)
	for _, marker := range []string{"G:", "D:", "S:"} {
		if i := strings.Index(rest, marker); i >= 0 && i < end {
			end = i
		}
	}
	return strings.TrimSpace(rest[:end])
}
