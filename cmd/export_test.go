package cmd

// RegisterForTest exposes the command registration to the black-box test package.
//
// The alternative was an internal test file, which `testpackage` rejects and which would
// have made this package's only white-box test the one about its public surface. This
// file is compiled only under `go test`, so the shipped package gains nothing — and
// exposing the *real* registration is the point: a test that built its own command list
// would be a hand-maintained allowlist of exactly the kind this codebase has twice
// deleted after it fell behind.
var RegisterForTest = register
