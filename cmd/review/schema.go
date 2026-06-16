package review

import (
	"github.com/UtakataKyosui/gh-wheel/internal/reviewschema"
	"github.com/spf13/cobra"
)

// NewSchemaCmd returns the `gh wheel review schema` subcommand.
func NewSchemaCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Print the JSON Schema for review output to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := cmd.OutOrStdout().Write(reviewschema.Schema())
			return err
		},
	}
}
