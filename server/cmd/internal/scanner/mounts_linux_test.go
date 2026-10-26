package scanner

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseMountInfoReadsMountPointsAndTypes(t *testing.T) {
	table := `22 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
30 22 0:40 / /srv/media rw,relatime shared:5 - ext4 /dev/sdb1 rw
31 30 0:41 / /srv/media/nas\040share rw,relatime shared:6 master:2 - nfs4 nas:/export rw
malformed line
`
	got := ParseMountInfo([]byte(table))
	want := []Mount{{Point: "/", Type: "ext4"}, {Point: "/srv/media", Type: "ext4"}, {Point: "/srv/media/nas share", Type: "nfs4"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseMountInfo = %+v, want %+v", got, want)
	}
	points := []string{"/", "/srv/media", "/srv/media/nas share"}
	beneath := MountPointsBeneath(points, "/srv/media")
	if !reflect.DeepEqual(beneath, []string{".", "nas share"}) {
		t.Fatalf("MountPointsBeneath = %q", beneath)
	}
	beneath = MountPointsBeneath(points, "/srv/media2")
	if beneath != nil {
		t.Fatalf("a name that only shares a prefix is not beneath: %q", beneath)
	}
}

// An overlay mount with many layers writes one very long line; the mounts
// after it are still read.
func TestParseMountInfoReadsPastAVeryLongLine(t *testing.T) {
	long := "40 22 0:50 / /var/lib/docker/overlay rw - overlay overlay rw,lowerdir=" + strings.Repeat("/l", 40000) + "\n"
	table := "22 1 8:2 / / rw - ext4 /dev/sda2 rw\n" + long + "41 22 8:33 / /srv/media/disk2 rw - ext4 /dev/sdc1 rw\n"
	got := ParseMountInfo([]byte(table))
	if len(got) != 3 || got[2].Point != "/srv/media/disk2" {
		t.Fatalf("ParseMountInfo = %d mounts, last %+v", len(got), got[len(got)-1])
	}
}

func TestUnescapeMountPoint(t *testing.T) {
	for field, want := range map[string]string{
		`/mnt/a\040b`:        "/mnt/a b",
		`/mnt/tab\011`:       "/mnt/tab\t",
		`/mnt/back\134slash`: `/mnt/back\slash`,
		`/mnt/plain`:         "/mnt/plain",
		`/mnt/short\04`:      `/mnt/short\04`,
	} {
		got := unescapeMountPoint(field)
		if got != want {
			t.Errorf("unescapeMountPoint(%q) = %q, want %q", field, got, want)
		}
	}
}
