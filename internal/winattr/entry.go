package winattr

import "strconv"

// Entry holds Windows security descriptor (SDDL), file attributes and timestamps.
type Entry struct {
	SDDL       string `json:"sddl,omitempty"`
	Attributes uint32 `json:"attributes,omitempty"`
	MtimeNS    int64  `json:"mtime_ns,omitempty"`
	CtimeNS    int64  `json:"ctime_ns,omitempty"`
	AtimeNS    int64  `json:"atime_ns,omitempty"`
}

// HasTimes reports whether any timestamp is stored.
func (e Entry) HasTimes() bool {
	return e.MtimeNS != 0 || e.CtimeNS != 0 || e.AtimeNS != 0
}

// HasMeta reports whether entry carries restorable metadata.
func (e Entry) HasMeta() bool {
	return e.SDDL != "" || e.Attributes != 0 || e.HasTimes()
}

// Hash returns a stable fingerprint for change detection.
func (e Entry) Hash() string {
	if !e.HasMeta() {
		return ""
	}
	return e.SDDL + "|" +
		strconv.FormatUint(uint64(e.Attributes), 10) + "|" +
		strconv.FormatInt(e.MtimeNS, 10) + "|" +
		strconv.FormatInt(e.CtimeNS, 10) + "|" +
		strconv.FormatInt(e.AtimeNS, 10)
}
