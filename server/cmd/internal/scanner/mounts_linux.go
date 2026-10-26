package scanner

import (
	"bytes"
	"os"
	"strconv"
	"strings"
)

// Mounts reads the mount table. It never touches a mounted filesystem, so a
// share whose server is down cannot stall it.
func Mounts() ([]Mount, error) {
	table, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	return ParseMountInfo(table), nil
}

// ParseMountInfo reads a /proc/self/mountinfo table: field 5 is the mount
// point, and the filesystem type follows the "-" that ends the optional
// fields. Malformed lines are skipped.
func ParseMountInfo(table []byte) []Mount {
	var mounts []Mount
	// Split rather than scanned: an overlay mount's line can outgrow a
	// Scanner's token limit, which would drop every mount after it.
	for _, line := range bytes.Split(table, []byte("\n")) {
		fields := strings.Fields(string(line))
		if len(fields) < 5 {
			continue
		}
		for i := 5; i < len(fields)-1; i++ {
			if fields[i] == "-" {
				mounts = append(mounts, Mount{Point: unescapeMountPoint(fields[4]), Type: fields[i+1]})
				break
			}
		}
	}
	return mounts
}

// unescapeMountPoint decodes the octal escapes mountinfo uses for space, tab,
// newline and backslash.
func unescapeMountPoint(field string) string {
	escaped := strings.Contains(field, `\`)
	if !escaped {
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
