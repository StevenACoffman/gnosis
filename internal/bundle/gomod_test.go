package bundle_test

import "os"

// goMod reads the module file from the repository root.
//
// The path is written relative to this test file rather than derived from
// os.Getwd, per rules.md: a fixture located by the working directory breaks the
// moment the test is run from anywhere else.
func goMod() (string, error) {
	b, err := os.ReadFile("../../go.mod")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
