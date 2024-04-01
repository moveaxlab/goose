package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// Main is the entry point for the CLI.
//
// If an error is returned, it is printed to stderr and the process exits with a non-zero exit code.
// The process is also canceled when an interrupt signal is received. This function and does not
// return.
func Main() {
	ctx, stop := newContext()
	defer stop()

	go func() {
		defer stop()
		if err := Run(ctx, os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}()
	select {
	case <-ctx.Done():
		stop()
	}
	// TODO(mf): should we exit or return a code and let the caller decide?
	os.Exit(0)
}

// Run runs the CLI with the provided arguments. The arguments should not include the command name
// itself, only the arguments to the command, e.g., os.Args[1:].
//
// RunOptions can be used to customize the behavior of the CLI, such as setting the environment,
// redirecting stdout and stderr, and providing a custom filesystem such as embed.FS.
func Run(ctx context.Context, args []string, opts ...RunOptions) error {
	state := &state{
		environ: os.Environ(),
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		fsys:    func(dir string) (fs.FS, error) { return os.DirFS(dir), nil },
	}
	for _, opt := range opts {
		if err := opt.apply(state); err != nil {
			return err
		}
	}
	return run(ctx, state, args)
}

func newContext() (context.Context, context.CancelFunc) {
	signals := []os.Signal{os.Interrupt}
	if runtime.GOOS != "windows" {
		signals = append(signals, syscall.SIGTERM)
	}
	return signal.NotifyContext(context.Background(), signals...)
}
