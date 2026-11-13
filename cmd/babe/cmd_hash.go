package main

import (
	"github.com/spf13/cobra"
)

func newHashCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "hash <file-path>",
		Short:   "Compute MD5, SHA-1 & SHA-256 file hashes",
		Long:    "Compute and display MD5, SHA-1, and SHA-256 cryptographic hashes for a file.",
		GroupID: "pkg",
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			printFileHashes(args[0])
		},
	}
}
