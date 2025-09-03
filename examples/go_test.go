//go:build go || all
// +build go all

package examples

import (
	"testing"
)

func TestGoExampleLifecycle(t *testing.T) {
	// Skip Go test due to environment issues with Go version detection in test containers
	t.Skip("Go test skipped due to environment issues with Go version detection")
}
