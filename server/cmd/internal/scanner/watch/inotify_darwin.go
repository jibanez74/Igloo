package watch

import "sync"

// macOS is a development platform: kqueue would hold a descriptor open for
// every watched file, so every library polls instead.
func newNotifier(*sync.WaitGroup) (notifier, error) {
	return nil, errNotifierUnsupported
}

func networkFilesystem(string) (string, error) {
	return "", nil
}

func watchLimitReached(error) bool {
	return false
}
