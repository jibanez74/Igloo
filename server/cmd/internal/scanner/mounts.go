package scanner

import "path/filepath"

// Mount is one entry of the kernel's mount table.
type Mount struct {
	Point string
	Type  string
}

// MountPoints lists the mount points in the mount table.
func MountPoints() ([]string, error) {
	mounts, err := Mounts()
	if err != nil {
		return nil, err
	}
	points := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		points = append(points, mount.Point)
	}
	return points, nil
}

// MountPointsBeneath returns the mount points at or beneath root, relative to
// it; "." is root itself. root must have its symlinks resolved, as the mount
// table's paths do.
func MountPointsBeneath(points []string, root string) []string {
	var beneath []string
	for _, point := range points {
		within := PathWithin(root, point)
		if !within {
			continue
		}
		relative, err := filepath.Rel(root, point)
		if err == nil {
			beneath = append(beneath, relative)
		}
	}
	return beneath
}
