package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3/internal/cli"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestRun(t *testing.T) {
	t.Run("version", func(t *testing.T) {
		stdout, stderr, err := runCommand("--version")
		require.NoError(t, err)
		require.Empty(t, stderr)
		require.Equal(t, stdout, "goose version: devel (unknown revision)\n")
	})
	t.Run("with_filesystem", func(t *testing.T) {
		fsys := fstest.MapFS{
			"migrations/001_foo.sql": {Data: []byte(`-- +goose Up`)},
		}
		command := "status --dir=migrations --dbstring=sqlite3://:memory:"
		err := cli.Run(context.Background(), strings.Split(command, " "), cli.WithFilesystem(fsys.Sub))
		require.NoError(t, err)
	})
}

func runCommand(args ...string) (string, string, error) {
	stdout, stderr := bytes.NewBuffer(nil), bytes.NewBuffer(nil)
	err := cli.Run(context.Background(), args, cli.WithStdout(stdout), cli.WithStderr(stderr))
	return stdout.String(), stderr.String(), err
}
