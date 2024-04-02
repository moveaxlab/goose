package cli

import (
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffval"
)

var requiredFlags = map[string]bool{
	"dir":      true,
	"dbstring": true,
}

func newDirFlag(s *string) ff.FlagConfig {
	return ff.FlagConfig{
		LongName:    "dir",
		Usage:       "directory with migration files",
		NoDefault:   true,
		Value:       ffval.NewValue(s),
		Placeholder: "string",
	}
}

func newDBStringFlag(s *string) ff.FlagConfig {
	return ff.FlagConfig{
		LongName:    "dbstring",
		Usage:       "connection string for the database",
		NoDefault:   true,
		Value:       ffval.NewValue(s),
		Placeholder: "string",
	}
}

func newJSONFlag(b *bool) ff.FlagConfig {
	return ff.FlagConfig{
		LongName: "json",
		Usage:    "output as JSON",
		Value:    ffval.NewValue(b),
	}
}
