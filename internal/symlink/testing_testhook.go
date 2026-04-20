package symlink

import "testing"

var swapLinkOverride func(tmpLink, linkPath string) error

func TestHookSwapLink(t *testing.T, fn func(tmpLink, linkPath string) error) func() {
	t.Helper()
	prev := swapLinkOverride
	swapLinkOverride = fn
	return func() {
		swapLinkOverride = prev
	}
}
