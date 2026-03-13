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

// scanCgroup iterates procRoot and calls yield for each PID whose cgroup path
// contains containerName. It skips PID 1 and the calling process's own PID.
func scanCgroup(procRoot, containerName string, yield func(pid int) bool) error {
	entries, err := os.ReadDir(procRoot)

	if err != nil {
		return fmt.Errorf("cannot read %s: %w", procRoot, err)
	}

	self := os.Getpid()

	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == 1 || pid == self {
			continue
		}

		cgroup, err := os.ReadFile(filepath.Join(procRoot, e.Name(), "cgroup")) //nolint:gosec // G304: path is constructed from /proc + numeric PID dir
		if err != nil {
			continue
		}

		if strings.Contains(string(cgroup), containerName) {
			if !yield(pid) {
				return nil
			}
		}
	}

	return nil
}

// ScanProc scans procRoot for the first PID whose cgroup path contains
// containerName. Accepting procRoot makes this function unit-testable.
func ScanProc(procRoot, containerName string) (int, error) {
	found := 0
	err := scanCgroup(procRoot, containerName, func(pid int) bool {
		found = pid
		return false // stop after the first match
	})
	if err != nil {
		return 0, err
	}
	if found == 0 {
		return 0, fmt.Errorf("no process found for container %q", containerName)
	}

	return found, nil
}

// ScanProcLowest scans procRoot and returns the smallest PID whose cgroup path
// contains containerName. Accepting procRoot makes this function unit-testable.
func ScanProcLowest(procRoot, containerName string) (int, error) {
	lowest := 0
	err := scanCgroup(procRoot, containerName, func(pid int) bool {
		if lowest == 0 || pid < lowest {
			lowest = pid
		}

		return true // keep scanning for a smaller PID
	})
	if err != nil {
		return 0, err
	}
	if lowest == 0 {
		return 0, fmt.Errorf("no process found for container %q", containerName)
	}

	return lowest, nil
}
