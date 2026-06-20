package doctor

// CheckGateForTest exposes checkGate to the black-box _test package. It lives in
// an export_test.go file so the shim is compiled only for tests, never into the
// release binary.
func CheckGateForTest(env Env) Result { return checkGate(env) }
