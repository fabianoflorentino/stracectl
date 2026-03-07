// Package discover locates the PID of a container process inside a shared
// PID namespace (Kubernetes sidecar with shareProcessNamespace: true).
package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ContainerPID returns the first PID whose cgroup path contains containerName.
// It reads from the live /proc filesystem.
func ContainerPID(containerName string) (int, error) {
	return ScanProc("/proc", containerName)
}

// LowestPIDInContainer returns the smallest PID whose cgroup path contains
// containerName — typically the init process of that container.
// It reads from the live /proc filesystem.
func LowestPIDInContainer(containerName string) (int, error) {
	return ScanProcLowest("/proc", containerName)
}

// ScanProc scans procRoot for the first PID whose cgroup path contains
// containerName. Accepting procRoot makes this function unit-testable.
func ScanProc(procRoot, containerName string) (int, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %w", procRoot, err)
	}

	self := os.Getpid()
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == 1 || pid == self {
			continue
		}

		cgroup, err := os.ReadFile(filepath.Join(procRoot, e.Name(), "cgroup"))
		if err != nil {
			continue
		}

		if strings.Contains(string(cgroup), containerName) {
			return pid, nil
		}
	}

	return 0, fmt.Errorf("no process found for container %q", containerName)
}

// ScanProcLowest scans procRoot and returns the smallest PID whose cgroup path
// contains containerName. Accepting procRoot makes this function unit-testable.
func ScanProcLowest(procRoot, containerName string) (int, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %w", procRoot, err)
	}

	self := os.Getpid()
	lowest := 0
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == 1 || pid == self {
			continue
		}

		cgroup, err := os.ReadFile(filepath.Join(procRoot, e.Name(), "cgroup"))
		if err != nil {
			continue
		}

		if strings.Contains(string(cgroup), containerName) {
			if lowest == 0 || pid < lowest {
				lowest = pid
			}
		}
	}

	if lowest == 0 {
		return 0, fmt.Errorf("no process found for container %q", containerName)
	}
	return lowest, nil
}
