package pail

// This is just a writer that does nothing, it is implemented for when the
// local bucket is set with dryRun to true.
type mockWriteCloser struct{}

// These functions do not do anything.
func (m *mockWriteCloser) Write(p []byte) (n int, err error) { return len(p), nil }
func (m *mockWriteCloser) Close() error                      { return nil }
