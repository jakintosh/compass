package main

import (
	"git.sr.ht/~jakintosh/command-go/pkg/args"
	"git.sr.ht/~jakintosh/compass/internal/server"
)

var serveCmd = &args.Command{
	Name: "serve",
	Help: "run the Compass web server",
	Options: []args.Option{
		{
			Long: "dev",
			Type: args.OptionTypeFlag,
			Help: "run without a consent server",
		},
		{
			Long: "addr",
			Type: args.OptionTypeParameter,
			Help: "listen address (env: ADDR)",
		},
		{
			Long: "data-dir",
			Type: args.OptionTypeParameter,
			Help: "runtime data directory (env: DATA_DIR)",
		},
		{
			Long: "consent-url",
			Type: args.OptionTypeParameter,
			Help: "consent server URL (env: CONSENT_URL)",
		},
		{
			Long: "consent-pubkey",
			Type: args.OptionTypeParameter,
			Help: "consent server public key PEM (env: CONSENT_PUBKEY)",
		},
		{
			Long: "integration-name",
			Type: args.OptionTypeParameter,
			Help: "consent integration name (env: CONSENT_INTEGRATION)",
		},
		{
			Long: "public-url",
			Type: args.OptionTypeParameter,
			Help: "public base URL for this Compass instance (env: PUBLIC_URL)",
		},
	},
	Handler: func(i *args.Input) error {
		opts := server.Options{
			Dev:             i.GetFlag("dev"),
			Addr:            cascadeParameter(i, "addr", "ADDR", ":8080"),
			DataDir:         cascadeParameter(i, "data-dir", "DATA_DIR", "data"),
			ConsentURL:      cascadeParameter(i, "consent-url", "CONSENT_URL"),
			ConsentPubkey:   cascadeParameter(i, "consent-pubkey", "CONSENT_PUBKEY"),
			IntegrationName: cascadeParameter(i, "integration-name", "CONSENT_INTEGRATION", "compass"),
			PublicURL:       cascadeParameter(i, "public-url", "PUBLIC_URL"),
		}
		return server.Serve(opts)
	},
}
