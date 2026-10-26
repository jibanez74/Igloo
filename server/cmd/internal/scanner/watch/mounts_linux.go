package watch

import (
	"strings"

	"igloo/cmd/internal/scanner"
)

// networkFilesystem names the filesystem that holds root, or that is mounted
// beneath it, when inotify cannot see its changes: they come from another
// machine or bypass this kernel's view. It reads the mount table rather than
// the filesystem, so classifying a library never touches a share whose server
// is down. Empty means inotify can watch the whole library.
func networkFilesystem(root string) (string, error) {
	mounts, err := scanner.Mounts()
	if err != nil {
		return "", err
	}
	return networkFilesystemIn(mounts, root), nil
}

// networkFilesystemIn applies networkFilesystem to a mount table.
func networkFilesystemIn(mounts []scanner.Mount, root string) string {
	holder, holderType := "", ""
	for _, mount := range mounts {
		beneath := mount.Point != root && scanner.PathWithin(root, mount.Point)
		if beneath && untrustedFilesystem(mount.Type) {
			return mount.Type
		}
		holds := scanner.PathWithin(mount.Point, root) && len(mount.Point) >= len(holder)
		if holds {
			holder, holderType = mount.Point, mount.Type
		}
	}
	if untrustedFilesystem(holderType) {
		return holderType
	}
	return ""
}

// untrustedFilesystem reports network shares and FUSE, mergerfs among them,
// whose branches change underneath it.
func untrustedFilesystem(fsType string) bool {
	switch fsType {
	case "nfs", "nfs4", "cifs", "smb3", "smbfs", "9p", "ceph", "fuse", "fuseblk":
		return true
	}
	return strings.HasPrefix(fsType, "fuse.")
}
