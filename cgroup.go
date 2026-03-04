// Copyright 2017 Bobby Powers. All rights reserved.
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// MemoryStat holds memory statistics from the cgroup
type MemoryStat struct {
	Usage   uint64
	Kernel  uint64 // Separate kernel memory tracking (v1 only; in v2 it's included in Usage)
	Hugetlb uint64 // Hugetlb usage (not included in standard RSS)
}

// CgroupManager provides an abstraction over cgroups v1 and v2
type CgroupManager interface {
	Add(pid int) error
	Stat() (*MemoryStat, error)
	Delete() error
}

// cgroupV1Manager wraps cgroup1.Cgroup
type cgroupV1Manager struct {
	cgroup cgroup1.Cgroup
}

// cgroupV2Manager wraps cgroup2.Manager
type cgroupV2Manager struct {
	manager *cgroup2.Manager
}

// NewCgroupManager creates a cgroup manager for the current system
// It auto-detects whether to use v1 or v2 based on system configuration
func NewCgroupManager(path string, memLimit int64) (CgroupManager, error) {
	mode := cgroups.Mode()

	switch mode {
	case cgroups.Unified:
		return newV2Manager(path, memLimit)
	case cgroups.Legacy, cgroups.Hybrid:
		return newV1Manager(path, memLimit)
	default:
		return nil, fmt.Errorf("cgroups not available on this system (mode: %v)", mode)
	}
}

// CgroupMode returns a string describing the cgroup mode
func CgroupMode() string {
	mode := cgroups.Mode()
	switch mode {
	case cgroups.Unified:
		return "unified (v2)"
	case cgroups.Legacy:
		return "legacy (v1)"
	case cgroups.Hybrid:
		return "hybrid (v1+v2)"
	default:
		return "unavailable"
	}
}

func newV1Manager(path string, memLimit int64) (*cgroupV1Manager, error) {
	resources := &specs.LinuxResources{
		Memory: &specs.LinuxMemory{
			Limit: &memLimit,
		},
	}

	cg, err := cgroup1.New(cgroup1.StaticPath(path), resources)
	if err != nil {
		return nil, fmt.Errorf("cgroup1.New: %w", err)
	}

	return &cgroupV1Manager{cgroup: cg}, nil
}

func (m *cgroupV1Manager) Add(pid int) error {
	return m.cgroup.Add(cgroup1.Process{Pid: pid})
}

func (m *cgroupV1Manager) Stat() (*MemoryStat, error) {
	stats, err := m.cgroup.Stat(cgroup1.IgnoreNotExist)
	if err != nil {
		return nil, fmt.Errorf("cgroup.Stat: %w", err)
	}
	if stats == nil || stats.Memory == nil {
		return nil, fmt.Errorf("cgroup.Stat returned nil memory stats")
	}

	return &MemoryStat{
		Usage:  stats.Memory.Usage.Usage,
		Kernel: stats.Memory.Kernel.Usage,
	}, nil
}

func (m *cgroupV1Manager) Delete() error {
	return m.cgroup.Delete()
}

func newV2Manager(path string, memLimit int64) (*cgroupV2Manager, error) {
	// In cgroups v2, we use the unified hierarchy under /sys/fs/cgroup
	// The path should be relative to the cgroup root
	resources := &cgroup2.Resources{
		Memory: &cgroup2.Memory{
			Max: &memLimit,
		},
		HugeTlb: &cgroup2.HugeTlb{},
	}

	// Remove leading slash if present for v2
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	mgr, err := cgroup2.NewManager("/sys/fs/cgroup", "/"+path, resources)
	if err != nil {
		return nil, fmt.Errorf("cgroup2.NewManager: %w", err)
	}

	return &cgroupV2Manager{manager: mgr}, nil
}

func (m *cgroupV2Manager) Add(pid int) error {
	return m.manager.AddProc(uint64(pid))
}

func (m *cgroupV2Manager) Stat() (*MemoryStat, error) {
	stats, err := m.manager.Stat()
	if err != nil {
		return nil, fmt.Errorf("manager.Stat: %w", err)
	}
	if stats == nil || stats.Memory == nil {
		return nil, fmt.Errorf("manager.Stat returned nil memory stats")
	}

	// In cgroups v2, kernel memory is included in the total usage and cannot
	// be queried separately. The separate kmem controller was removed in v2.
	// See: https://github.com/opencontainers/runtime-spec/issues/1005
	var hugetlb uint64
	for _, h := range stats.Hugetlb {
		hugetlb += h.Current
	}
	return &MemoryStat{
		Usage:   stats.Memory.Usage,
		Kernel:  0, // Not separately tracked in v2 (included in Usage)
		Hugetlb: hugetlb,
	}, nil
}

func (m *cgroupV2Manager) Delete() error {
	return m.manager.Delete()
}
