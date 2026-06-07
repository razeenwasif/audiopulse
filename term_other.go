//go:build !linux

package main

// terminalCellAspect cannot be queried on this platform; the default is used.
func terminalCellAspect() (ratio float64, ok bool) { return 0, false }
