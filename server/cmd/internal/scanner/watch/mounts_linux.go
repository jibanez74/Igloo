package watch

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"

	"igloo/cmd/internal/scanner"
)

// networkFilesystem names the filesystem that holds root, or that is mounted
// beneath it, when inotify cannot see its changes: they come from another
// machine or bypass this kernel's view. It reads the mount table rather than
// the filesystem, so classifying a library never touches a share whose server
// is down. Empty means inotify can watch the whole library.
func networkFilesystem(root string) (string, error) {
	table, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	return networkFilesystemIn(table, root), nil
}

// networkFilesystemIn applies networkFilesystem to a mountinfo table.
func networkFilesystemIn(table []byte, root string) string {
	holder, holderType := "", ""
	lines := bufio.NewScanner(bytes.NewReader(table))
	for lines.Scan() {
		mountPoint, fsType, ok := parseMountInfo(lines.Text())
		if !ok {
			continue
		}
		beneath := mountPoint != root && scanner.PathWithin(root, mountPoint)
		if beneath && untrustedFilesystem(fsType) {
			return fsType
		}
		holds := scanner.PathWithin(mountPoint, root) && len(mountPoint) >= len(holder)
		if holds {
			holder, holderType = mountPoint, fsType
		}
	}
	if untrustedFilesystem(holderType) {
		return holderType
	}
	return ""
}

// parseMountInfo reads the mount point and filesystem type of one
// /proc/self/mountinfo line: field 5 is the mount point, and the type follows
// the "-" that ends the optional fields.
func parseMountInfo(line string) (mountPoint, fsType string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return "", "", false
	}
	for i := 5; i < len(fields)-1; i++ {
		if fields[i] == "-" {
			return unescapeMountPoint(fields[4]), fields[i+1], true
		}
	}
	return "", "", false
}

// unescapeMountPoint decodes the octal escapes mountinfo uses for space, tab,
// newline and backslash.
func unescapeMountPoint(field string) string {
	if !strings.Contains(field, `\`) {
		return field
	}
	var decoded strings.Builder
	for i := 0; i < len(field); i++ {
		if field[i] == '\\' && i+3 < len(field) {
			value, err := strconv.ParseUint(field[i+1:i+4], 8, 8)
			if err == nil {
				decoded.WriteByte(byte(value))
				i += 3
				continue
			}
		}
		decoded.WriteByte(field[i])
	}
	return decoded.String()
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
