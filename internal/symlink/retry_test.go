package symlink

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetrySwap_RetriesPermissionErrors(t *testing.T) {
	attempts := 0
	var sleeps []time.Duration

	err := retrySwap(3, func(d time.Duration) { sleeps = append(sleeps, d) }, func() error {
		attempts++
		if attempts == 1 {
			return os.ErrPermission
		}
		return nil
	}, func(err error) bool {
		return os.IsPermission(err)
	})

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, []time.Duration{25 * time.Millisecond}, sleeps)
}
