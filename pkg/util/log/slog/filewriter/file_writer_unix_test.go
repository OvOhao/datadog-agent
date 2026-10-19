// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

//go:build !windows

package filewriter

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setUmask gives the test the umask of most services, 022, under which a
// file gets the mode it is created with.
func setUmask(t *testing.T) {
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })
}

func TestRollingFileWriterFileMode(t *testing.T) {
	setUmask(t)
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	maxSize := int64(100)
	writer, err := NewRollingFileWriterSize(logPath, maxSize, 5, RollingNameModePostfix)
	require.NoError(t, err)
	defer writer.Close()

	// Write past the maximum size, then once more to roll the file.
	_, err = writer.Write([]byte(strings.Repeat("A", int(maxSize)+10)))
	require.NoError(t, err)
	_, err = writer.Write([]byte("trigger roll\n"))
	require.NoError(t, err)

	for _, path := range []string{logPath, logPath + ".1"} {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0640), info.Mode().Perm(), path)
	}
}

func TestRollingFileWriterTightensExistingFile(t *testing.T) {
	setUmask(t)
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")
	require.NoError(t, os.WriteFile(logPath, []byte("old\n"), 0644))

	writer, err := NewRollingFileWriterSize(logPath, 1024, 5, RollingNameModePostfix)
	require.NoError(t, err)
	defer writer.Close()

	_, err = writer.Write([]byte("test\n"))
	require.NoError(t, err)

	info, err := os.Stat(logPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm())
}
