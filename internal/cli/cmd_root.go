package cli

import "github.com/peterbourgon/ff/v4"

type rootCmd struct {
	flags   *ff.FlagSet
	command *ff.Command

	version bool
}
