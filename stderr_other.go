//go:build !linux

package main

// silenceNativeStderr is a no-op on platforms that don't have ALSA's
// fd-2 diagnostic noise (e.g. macOS Core Audio).
func silenceNativeStderr() (restore func()) { return func() {} }
