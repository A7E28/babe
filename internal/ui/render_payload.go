package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/A7E28/babe/internal/image"
)

func RenderPayloadInfo(info *image.PayloadInfo) {
	RenderPayloadInfoTo(os.Stdout, info)
}

func RenderPayloadInfoTo(w io.Writer, info *image.PayloadInfo) {
	fmt.Fprintln(w)
	verLabel := fmt.Sprintf("%s, v%d", info.SourceType, info.FormatVersion)
	if info.SourceType == "legacy-zip" {
		verLabel = "legacy non-A/B zip"
	}
	fmt.Fprintf(w, "  %s %s %s\n", Gray("Source:         "), White(sanitizeTerminalText(info.FilePath)), Gray("("+verLabel+")"))
	otaLabel := Green("Full OTA")
	if info.SourceType == "legacy-zip" {
		if info.OTAType != "full" {
			otaLabel = Yellow("Legacy Non-A/B Incremental Block OTA")
		} else {
			otaLabel = Cyan("Legacy Non-A/B Block OTA")
		}
	} else if info.OTAType != "full" {
		otaLabel = Yellow(fmt.Sprintf("Delta / Incremental OTA (minor v%d)", info.MinorVersion))
	}
	fmt.Fprintf(w, "  %s %s\n", Gray("OTA Type:       "), otaLabel)
	fmt.Fprintf(w, "  %s %s\n", Gray("Block Size:     "), White(fmt.Sprintf("%d bytes", info.BlockSize)))
	if info.SecurityPatchLevel != "" {
		fmt.Fprintf(w, "  %s %s\n", Gray("Security Patch: "), White(sanitizeTerminalText(info.SecurityPatchLevel)))
	}
	dev := info.OTAMetadata["pre-device"]
	if dev == "" {
		dev = info.OTAMetadata["post-device"]
	}
	if dev != "" {
		fmt.Fprintf(w, "  %s %s\n", Gray("Device:         "), BoldCyan(sanitizeTerminalText(dev)))
	}
	if build := info.OTAMetadata["post-build"]; build != "" {
		fmt.Fprintf(w, "  %s %s\n", Gray("Build:          "), Gray(sanitizeTerminalText(build)))
	}
	totalLabel := fmt.Sprintf("(%.2f MB uncompressed)", float64(info.TotalSize)/(1024*1024))
	if info.TotalSize == ^uint64(0) {
		totalLabel = "(size overflow)"
	}
	fmt.Fprintf(w, "  %s %s %s\n", Gray("Partitions:     "), White(fmt.Sprintf("%d total", len(info.Partitions))), Gray(totalLabel))

	nameWidth := 18
	for _, p := range info.Partitions {
		if n := utf8.RuneCountInString(sanitizeTerminalText(p.Name)); n > nameWidth {
			nameWidth = n
		}
	}
	header := fmt.Sprintf("%-*s  %s    %s  %s", nameWidth, "NAME", "SIZE (MB)", "CODECS          ", "STATUS")
	fmt.Fprintf(w, "\n  %s\n", Gray(header))
	for _, p := range info.Partitions {
		mb := float64(p.Size) / (1024 * 1024)
		codecs := strings.Join(p.Codecs, ",")
		if codecs == "" {
			codecs = "-"
		}
		status := Green("[OK]")
		if !p.Extractable {
			status = Yellow("[DELTA]")
			if reason := sanitizeTerminalText(p.UnsupportedReason); reason != "" {
				status += " " + Gray(reason)
			}
		}
		nameCol := BoldCyan(fmt.Sprintf("%-*s", nameWidth, sanitizeTerminalText(p.Name)))
		sizeCol := White(fmt.Sprintf("%9.2f", mb))
		codecCol := Cyan(fmt.Sprintf("%-16s", sanitizeTerminalText(codecs)))
		fmt.Fprintf(w, "  %s  %s    %s  %s\n", nameCol, sizeCol, codecCol, status)
	}
}

func RenderPayloadExtractResult(res *image.PayloadExtractResult) {
	RenderPayloadExtractResultTo(os.Stdout, res)
}

func RenderPayloadExtractResultTo(w io.Writer, res *image.PayloadExtractResult) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s Extracted %d partition(s) using %d worker(s) in %d ms:\n",
		BoldCyan("[INFO]"), len(res.Extracted), res.Jobs, res.DurationMs)
	for _, item := range res.Extracted {
		badge := Green("[VERIFIED]")
		if !item.Verified {
			if res.SourceType == "legacy-zip" {
				badge = Cyan("[EXTRACTED]")
			} else {
				badge = Yellow("[UNVERIFIED]")
			}
		}
		fmt.Fprintf(w, "  %s %-16s -> %s %s\n",
			badge,
			BoldCyan(sanitizeTerminalText(item.Partition)),
			White(sanitizeTerminalText(item.OutputPath)),
			Gray(fmt.Sprintf("(%.2f MB, %d ms)", float64(item.Size)/(1024*1024), item.DurationMs)),
		)
		if item.SHA256 != "" {
			fmt.Fprintf(w, "     %s %s\n", Gray("SHA256:"), Gray(item.SHA256))
		}
		if item.BootInfo != nil {
			detail := fmt.Sprintf("v%d", item.BootInfo.HeaderVersion)
			var parts []string
			if osVer := sanitizeTerminalText(item.BootInfo.OSVersion); osVer != "" {
				parts = append(parts, "OS "+osVer)
			}
			if patch := sanitizeTerminalText(item.BootInfo.OSPatchLevel); patch != "" {
				parts = append(parts, "Patch "+patch)
			}
			if len(parts) > 0 {
				detail += " (" + strings.Join(parts, ", ") + ")"
			}
			fmt.Fprintf(w, "     %s %s\n", Gray("Boot Header:"), detail)
		}
	}
}

func sanitizeTerminalText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\x1b' || unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
