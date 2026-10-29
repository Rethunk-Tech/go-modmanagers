//go:build !windows

package vortex

func fileLocked(error) bool { return false }
