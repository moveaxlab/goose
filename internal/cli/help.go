package cli

import (
	"os"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
)

const (
	redColor = "#cc0000"
)

func createHelp(cmd *ff.Command) ffhelp.Help {
	var help ffhelp.Help
	if selected := cmd.GetSelected(); selected != nil {
		cmd = selected
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(redColor))
	render := func(s string) string {
		// TODO(mf): should we also support a global flag to disable color?
		if val := os.Getenv("NO_COLOR"); val != "" {
			if ok, err := strconv.ParseBool(val); err == nil && ok {
				return s
			}
		}
		return style.Render(s)
	}

	title := cmd.Name
	if cmd.ShortHelp != "" {
		title = title + " -- " + cmd.ShortHelp
	}
	help = append(help, ffhelp.NewSection(render("COMMAND"), title))

	if cmd.LongHelp != "" {
		section := ffhelp.NewUntitledSection(cmd.LongHelp)
		help = append(help, section)
	}

	if cmd.Usage != "" {
		help = append(help, ffhelp.NewSection(render("USAGE"), cmd.Usage))
	}

	if len(cmd.Subcommands) > 0 {
		section := ffhelp.NewSubcommandsSection(cmd.Subcommands)
		section.Title = render(section.Title)
		help = append(help, section)
	}

	for _, section := range ffhelp.NewFlagsSections(cmd.Flags) {
		section.Title = render(section.Title)
		help = append(help, section)
	}
	if sections, ok := additionalSections[cmd.Name]; ok {
		for _, section := range sections {
			section.Title = render(section.Title)
			help = append(help, section)
		}
	}

	return help
}

// additionalSections contains additional help sections for specific commands.
var additionalSections = map[string][]ffhelp.Section{
	"status": {
		{
			Title: "EXAMPLES",
			Lines: []string{
				`goose status --dir=migrations --dbstring=sqlite:./test.db`,
				`GOOSE_DIR=migrations GOOSE_DBSTRING=sqlite:./test.db goose status`,
			},
			LinePrefix: ffhelp.DefaultLinePrefix,
		},
	},
}
