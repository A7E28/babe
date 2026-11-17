package main

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/doctor"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/hash"
	"github.com/A7E28/babe/internal/logcat"
	"github.com/A7E28/babe/internal/tools"
)

type errorReport struct {
	Error string `json:"error"`
}

type versionReport struct {
	Version string `json:"version"`
}

type hashReport struct {
	File string `json:"file"`
	hash.MultiDigest
}

type doctorReport struct {
	Status    doctor.Status        `json:"status"`
	Checks    []doctor.CheckResult `json:"checks"`
	UdevFixed *bool                `json:"udev_fixed,omitempty"`
}

type deviceListReport struct {
	Count   int             `json:"count"`
	Devices []device.Device `json:"devices"`
}

type rebootReport struct {
	Serial   string `json:"serial"`
	Target   string `json:"target"`
	Rebooted bool   `json:"rebooted"`
}

type fastbootListReport struct {
	Count   int               `json:"count"`
	Devices []fastboot.Device `json:"devices"`
}

type slotReport struct {
	Serial     string `json:"serial"`
	ActiveSlot string `json:"active_slot"`
}

type flashReport struct {
	Serial    string `json:"serial"`
	Partition string `json:"partition"`
	Image     string `json:"image"`
	SHA256    string `json:"sha256,omitempty"`
	Flashed   bool   `json:"flashed"`
}

type apkInstallReport struct {
	Serial    string `json:"serial"`
	File      string `json:"file"`
	Installed bool   `json:"installed"`
}

type packageListReport struct {
	Serial   string   `json:"serial"`
	Count    int      `json:"count"`
	Packages []string `json:"packages"`
}

type apkPullReport struct {
	Serial     string `json:"serial"`
	Package    string `json:"package"`
	OutputPath string `json:"output_path"`
}

type crashLogReport struct {
	Serial     string         `json:"serial"`
	Count      int            `json:"count"`
	OutputFile string         `json:"output_file,omitempty"`
	Entries    []logcat.Entry `json:"entries"`
}

type udevReport struct {
	Installed bool `json:"installed"`
}

type platformToolsStatus struct {
	Installed  bool   `json:"installed"`
	Version    string `json:"version,omitempty"`
	BinaryPath string `json:"binary_path,omitempty"`
	InstallDir string `json:"install_dir,omitempty"`
	InPath     bool   `json:"in_path"`
}

type toolsStatusReport struct {
	PlatformTools platformToolsStatus      `json:"platform_tools"`
	Babe          *tools.BabeInstallStatus `json:"babe"`
}

type updateReport struct {
	Target          string `json:"target"`
	Installed       *bool  `json:"installed,omitempty"`
	CurrentVersion  string `json:"current_version,omitempty"`
	LatestVersion   string `json:"latest_version,omitempty"`
	Asset           string `json:"asset,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	Updated         bool   `json:"updated"`
	Path            string `json:"path,omitempty"`
	Warning         string `json:"warning,omitempty"`
}

func isJSON(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	v, err := cmd.Flags().GetBool("json")
	return err == nil && v
}

func hasJSONFlag(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--json" || strings.EqualFold(a, "--json=true") {
			return true
		}
	}
	return false
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeJSONLine(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func summarizeDoctorStatus(checks []doctor.CheckResult) doctor.Status {
	overall := doctor.StatusOK
	for _, c := range checks {
		if c.Status == doctor.StatusError {
			return doctor.StatusError
		}
		if c.Status == doctor.StatusWarn {
			overall = doctor.StatusWarn
		}
	}
	return overall
}
