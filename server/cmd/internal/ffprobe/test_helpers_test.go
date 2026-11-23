package ffprobe

import "testing"

func requireArgumentValue(t *testing.T, args []string, flag string, want string) {
	t.Helper()

	for index, arg := range args {
		if arg != flag {
			continue
		}
		if index+1 >= len(args) {
			t.Fatalf("%s has no value", flag)
		}
		if args[index+1] != want {
			t.Fatalf("%s = %q, want %q", flag, args[index+1], want)
		}
		return
	}

	t.Fatalf("%s argument not found in %q", flag, args)
}
