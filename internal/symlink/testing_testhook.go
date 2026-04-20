package symlink

import (
	"testing"
	"time"
)

var swapLinkOverride func(tmpLink, linkPath string) error
var swapLinkSleepOverride func(time.Duration)

func TestHookSwapLink(t *testing.T, fn func(tmpLink, linkPath string) error) func() {
	t.Helper()
	prev := swapLinkOverride
	swapLinkOverride = fn
	return func() {
		swapLinkOverride = prev
	}
}

func TestHookSwapSleep(t *testing.T, fn func(time.Duration)) func() {
	t.Helper()
	prev := swapLinkSleepOverride
	swapLinkSleepOverride = fn
	return func() {
		swapLinkSleepOverride = prev
	}
}
