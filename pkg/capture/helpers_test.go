package capture

import "os"

// writeFile is a thin shim over os.WriteFile so the test file doesn't
// need to import os directly. Keeping this tiny indirection lets us
// fake filesystem errors in the future without rewriting tests.
func writeFile(path string, perm os.FileMode) error {
	return os.WriteFile(path, nil, perm)
}
