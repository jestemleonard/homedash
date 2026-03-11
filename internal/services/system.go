package services

import (
	"github.com/jestemleonard/homedash/internal/models"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemService provides system statistics
type SystemService struct{}

// NewSystemService creates a new system service
func NewSystemService() *SystemService {
	return &SystemService{}
}

// GetStats returns current system statistics
func (s *SystemService) GetStats() (*models.SystemStats, error) {
	cpuStats, err := s.getCPUStats()
	if err != nil {
		return nil, err
	}

	memStats, err := s.getMemoryStats()
	if err != nil {
		return nil, err
	}

	diskStats, err := s.getDiskStats()
	if err != nil {
		return nil, err
	}

	return &models.SystemStats{
		CPU:    *cpuStats,
		Memory: *memStats,
		Disks:  diskStats,
	}, nil
}

func (s *SystemService) getCPUStats() (*models.CPUStats, error) {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}

	cores, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}

	var totalPercent float64
	if len(percentages) > 0 {
		totalPercent = percentages[0]
	}

	return &models.CPUStats{
		UsagePercent: totalPercent,
		Cores:        cores,
	}, nil
}

func (s *SystemService) getMemoryStats() (*models.MemoryStats, error) {
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	return &models.MemoryStats{
		Total:       vmem.Total,
		Used:        vmem.Used,
		Available:   vmem.Available,
		UsedPercent: vmem.UsedPercent,
	}, nil
}

func (s *SystemService) getDiskStats() ([]models.DiskStats, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	var stats []models.DiskStats
	seen := make(map[string]bool)

	for _, p := range partitions {
		// Skip duplicate mount points
		if seen[p.Mountpoint] {
			continue
		}
		seen[p.Mountpoint] = true

		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		// Skip small/system partitions
		if usage.Total < 1024*1024*1024 { // Less than 1GB
			continue
		}

		stats = append(stats, models.DiskStats{
			Path:        p.Mountpoint,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
		})
	}

	return stats, nil
}
