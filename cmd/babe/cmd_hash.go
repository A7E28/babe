package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/hash"
)

func newHashCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "hash <file-path>",
		Short:   "Compute MD5, SHA-1 & SHA-256 file hashes",
		Long:    "Compute and display MD5, SHA-1, and SHA-256 cryptographic hashes for a file.",
		GroupID: "pkg",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSON(cmd) {
				digests, err := hash.CalculateAllFileHashes(args[0])
				if err != nil {
					return fmt.Errorf("hash calculation failed: %w", err)
				}
				return writeJSON(cmd.OutOrStdout(), hashReport{
					File:        args[0],
					MultiDigest: digests,
				})
			}
			if err := printFileHashes(args[0]); err != nil {
				return fmt.Errorf("hash calculation failed: %w", err)
			}
			return nil
		},
	}
}
