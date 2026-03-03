package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xxxsen/md2cfhtml"
)

type cliOptions struct {
	input  string
	output string
}

func newRootCmd() *cobra.Command {
	opts := &cliOptions{}

	command := &cobra.Command{
		Use:   "conv",
		Short: "Convert Markdown file to Confluence HTML",
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.input == "" {
				return errors.New("--input is required")
			}
			if opts.output == "" {
				return errors.New("--output is required")
			}

			if err := md2cfhtml.ConvertFile(opts.input, opts.output); err != nil {
				return fmt.Errorf("convert markdown: %w", err)
			}
			return nil
		},
	}

	command.Flags().StringVar(&opts.input, "input", "", "input markdown file path")
	command.Flags().StringVar(&opts.output, "output", "", "output html file path")
	_ = command.MarkFlagRequired("input")
	_ = command.MarkFlagRequired("output")

	return command
}
