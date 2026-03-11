package models

// SystemStats contains all system statistics
type SystemStats struct {
	CPU    CPUStats    `json:"cpu"`
	Memory MemoryStats `json:"memory"`
	Disks  []DiskStats `json:"disks"`
}

// CPUStats contains CPU usage information
type CPUStats struct {
	UsagePercent float64   `json:"usage_percent"`
	Cores        int       `json:"cores"`
	PerCore      []float64 `json:"per_core,omitempty"`
}

// MemoryStats contains memory usage information
type MemoryStats struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Available   uint64  `json:"available"`
	UsedPercent float64 `json:"used_percent"`
}

// DiskStats contains disk usage information
type DiskStats struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}
