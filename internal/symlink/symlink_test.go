package symlink_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/symlink"
)

func TestInspect_NotExist(t *testing.T) {
	dir := t.TempDir()
	s, err := symlink.Inspect(filepath.Join(dir, "nonexistent"))
	require.NoError(t, err)
	assert.False(t, s.Exists)
	assert.False(t, s.IsSymlink)
	assert.False(t, s.IsDir)
	assert.Empty(t, s.Target)
	assert.False(t, s.Dangling)
}

func TestInspect_RealDir(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "realdir")
	require.NoError(t, os.Mkdir(subdir, 0o755))

	s, err := symlink.Inspect(subdir)
	require.NoError(t, err)
	assert.True(t, s.Exists)
	assert.False(t, s.IsSymlink)
	assert.True(t, s.IsDir)
	assert.Empty(t, s.Target)
	assert.False(t, s.Dangling)
}

func TestInspect_RealFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello"), 0o644))

	s, err := symlink.Inspect(f)
	require.NoError(t, err)
	assert.True(t, s.Exists)
	assert.False(t, s.IsSymlink)
	assert.False(t, s.IsDir)
}

func TestInspect_ValidSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	s, err := symlink.Inspect(link)
	require.NoError(t, err)
	assert.True(t, s.Exists)
	assert.True(t, s.IsSymlink)
	assert.Equal(t, target, s.Target)
	assert.False(t, s.Dangling)
}

func TestInspect_DanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "gone")
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))
	// target was never created — link is dangling

	s, err := symlink.Inspect(link)
	require.NoError(t, err)
	assert.True(t, s.Exists)
	assert.True(t, s.IsSymlink)
	assert.Equal(t, target, s.Target)
	assert.True(t, s.Dangling)
}

func TestSetAtomic_Fresh(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "profile")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "opencode")

	err := symlink.SetAtomic(target, link)
	require.NoError(t, err)

	// Verify symlink was created correctly.
	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target, got)

	// Verify temp file is gone.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".opm-tmp-", "temp file should be cleaned up after success")
	}
}

func TestSetAtomic_Replace(t *testing.T) {
	dir := t.TempDir()
	target1 := filepath.Join(dir, "profile1")
	target2 := filepath.Join(dir, "profile2")
	require.NoError(t, os.Mkdir(target1, 0o755))
	require.NoError(t, os.Mkdir(target2, 0o755))
	link := filepath.Join(dir, "opencode")

	// Create initial symlink.
	require.NoError(t, os.Symlink(target1, link))

	// Replace atomically — must not fail with EEXIST.
	err := symlink.SetAtomic(target2, link)
	require.NoError(t, err)

	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target2, got)
}

func TestSetAtomic_ReplaceDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	oldTarget := filepath.Join(dir, "profile-old")
	newTarget := filepath.Join(dir, "profile-new")
	require.NoError(t, os.Mkdir(oldTarget, 0o755))
	link := filepath.Join(dir, "opencode")
	require.NoError(t, os.Symlink(oldTarget, link))
	require.NoError(t, os.Rename(oldTarget, newTarget))

	err := symlink.SetAtomic(newTarget, link)
	require.NoError(t, err)

	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, newTarget, got)
}

func TestSetAtomic_ReplacesExistingLinkWhenSwapRenameCannotReplace(t *testing.T) {
	dir := t.TempDir()
	target1 := filepath.Join(dir, "profile1")
	target2 := filepath.Join(dir, "profile2")
	require.NoError(t, os.Mkdir(target1, 0o755))
	require.NoError(t, os.Mkdir(target2, 0o755))
	link := filepath.Join(dir, "opencode")
	require.NoError(t, os.Symlink(target1, link))

	restore := symlink.TestHookSwapLink(t, func(tmpLink, linkPath string) error {
		if err := os.Remove(linkPath); err != nil {
			return err
		}
		return os.Rename(tmpLink, linkPath)
	})
	t.Cleanup(restore)

	err := symlink.SetAtomic(target2, link)
	require.NoError(t, err)

	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target2, got)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.False(t, strings.HasPrefix(entry.Name(), ".opm-tmp-"), "temp file should be cleaned up after fallback replace")
	}
}

func TestSetAtomic_LeftoverTmpCleaned(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "profile")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "opencode")

	// Pre-create a leftover tmp file (simulates crash on previous run).
	tmpLink := filepath.Join(dir, ".opm-tmp-opencode")
	require.NoError(t, os.Symlink("/some/stale/target", tmpLink))

	// SetAtomic should handle it without error.
	err := symlink.SetAtomic(target, link)
	require.NoError(t, err)

	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target, got)
}

func TestSetAtomic_LeftoverRandomizedTmpCleaned(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "profile")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "opencode")

	liveProcess := exec.Command("sleep", "30")
	require.NoError(t, liveProcess.Start())
	t.Cleanup(func() {
		_ = liveProcess.Process.Kill()
		_, _ = liveProcess.Process.Wait()
	})

	malformed := filepath.Join(dir, ".opm-tmp-opencode-stale-1")
	stalePID := filepath.Join(dir, ".opm-tmp-opencode-999999-stale")
	livePID := filepath.Join(dir, ".opm-tmp-opencode-"+strconv.Itoa(liveProcess.Process.Pid)+"-stale")
	require.NoError(t, os.Symlink("/some/stale/target/malformed", malformed))
	require.NoError(t, os.Symlink("/some/stale/target/deadpid", stalePID))
	require.NoError(t, os.Symlink("/some/stale/target/livepid", livePID))

	err := symlink.SetAtomic(target, link)
	require.NoError(t, err)

	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target, got)

	_, err = os.Lstat(malformed)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Lstat(stalePID)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Lstat(livePID)
	require.NoError(t, err)
}

func TestSetAtomic_DoesNotDeleteRegularFileMatchingTempPrefix(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "profile")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "opencode")

	protected := filepath.Join(dir, ".opm-tmp-opencode-999999-note")
	require.NoError(t, os.WriteFile(protected, []byte("keep me"), 0o644))

	err := symlink.SetAtomic(target, link)
	require.NoError(t, err)

	data, err := os.ReadFile(protected)
	require.NoError(t, err)
	assert.Equal(t, "keep me", string(data))
}

func TestSetAtomic_DoesNotDeleteDirectoryMatchingTempPrefix(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "profile")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "opencode")

	protected := filepath.Join(dir, ".opm-tmp-opencode-999999-dir")
	require.NoError(t, os.Mkdir(protected, 0o755))

	err := symlink.SetAtomic(target, link)
	require.NoError(t, err)

	info, err := os.Stat(protected)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestSetAtomic_ConcurrentCallsShareNoTempName(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "opencode")

	const rounds = 25
	const workers = 16

	for round := 0; round < rounds; round++ {
		targets := make([]string, workers)
		for worker := 0; worker < workers; worker++ {
			target := filepath.Join(dir, fmt.Sprintf("profile-%d-%d", round, worker))
			require.NoError(t, os.Mkdir(target, 0o755))
			targets[worker] = target
		}

		start := make(chan struct{})
		results := make(chan error, workers)

		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func(target string) {
				defer wg.Done()
				<-start
				results <- symlink.SetAtomic(target, link)
			}(targets[worker])
		}

		close(start)
		wg.Wait()
		close(results)

		for err := range results {
			require.NoError(t, err)
		}

		got, err := os.Readlink(link)
		require.NoError(t, err)
		assert.Contains(t, targets, got)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".opm-tmp-", "temp file should be cleaned up after concurrent success")
	}
}
