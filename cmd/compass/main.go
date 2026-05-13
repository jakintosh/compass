package main

import (
	"os"

	"git.sr.ht/~jakintosh/command-go/pkg/args"
	"git.sr.ht/~jakintosh/command-go/pkg/version"
)

func main() {
	rootCmd.Parse()
}

var rootCmd = &args.Command{
	Name: "compass",
	Help: "manage Compass",
	Config: &args.Config{
		HelpOption: &args.HelpOption{
			Short: 'h',
			Long:  "help",
		},
	},
	Options: []args.Option{
		{
			Long: "verbose",
			Type: args.OptionTypeFlag,
			Help: "show detailed output",
		},
	},
	Subcommands: []*args.Command{
		version.Command(VersionInfo),
		serveCmd,
	},
}

func cascadeParameter(
	input *args.Input,
	optionName string,
	envName string,
	defaultValues ...string,
) string {
	value := input.GetParameterOr(optionName, "")
	if value != "" {
		return value
	}

	value = os.Getenv(envName)
	if value != "" {
		return value
	}

	for _, defaultValue := range defaultValues {
		if defaultValue != "" {
			return defaultValue
		}
	}
	return ""
}
