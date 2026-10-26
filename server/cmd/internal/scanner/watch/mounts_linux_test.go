package watch

import (
	"testing"

	"igloo/cmd/internal/scanner"
)

const testMountInfo = `22 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
30 22 0:40 / /srv/media rw,relatime shared:5 - ext4 /dev/sdb1 rw
31 30 0:41 / /srv/media/nas\040share rw,relatime shared:6 - nfs4 nas:/export rw
32 22 0:42 / /mnt/pool rw,relatime shared:7 - fuse.mergerfs pool rw
33 22 0:43 / /mnt/usb rw - vfat /dev/sdc1 rw
malformed line
`

func TestNetworkFilesystemReadsTheMountTable(t *testing.T) {
	tests := []struct {
		root, want string
	}{
		// A local root with a share mounted inside it polls.
		{"/srv/media", "nfs4"},
		{"/srv/media/nas share/Movies", "nfs4"},
		{"/srv/media/local", ""},
		{"/mnt/pool/Music", "fuse.mergerfs"},
		{"/mnt/usb", ""},
		{"/home/user/Movies", ""},
		// A name that only shares a prefix with a mount is not inside it.
		{"/srv/media2", ""},
	}
	for _, tc := range tests {
		got := networkFilesystemIn(scanner.ParseMountInfo([]byte(testMountInfo)), tc.root)
		if got != tc.want {
			t.Errorf("networkFilesystemIn(%q) = %q, want %q", tc.root, got, tc.want)
		}
	}
}
