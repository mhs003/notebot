package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mhs003/notebot/internal/store"
	"github.com/spf13/cobra"
)

const unitName = "notebot.service"

func unitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", unitName), nil
}

// findDaemon locates the notebotd binary: $PATH first (the installed copy),
// then beside the running nb executable.
func findDaemon() (string, error) {
	if p, err := exec.LookPath("notebotd"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs, nil
		}
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "notebotd")
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("notebotd not found (run `make install` first)")
}

func systemctl(args ...string) (string, error) {
	c := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	out, err := c.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func newInstallServiceCmd(dataDir *string) *cobra.Command {
	var noStart bool
	cmd := &cobra.Command{
		Use:   "install-service",
		Short: "Install and start the systemd user service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("systemctl"); err != nil {
				return fmt.Errorf("systemctl not found; systemd is required")
			}
			daemon, err := findDaemon()
			if err != nil {
				return err
			}
			cfg, err := store.LoadConfig(*dataDir)
			if err != nil {
				return err
			}
			path, err := unitPath()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}

			// Pin the exact data dir and port so the service does not depend on
			// the session environment matching the shell that ran `nb setup`.
			unit := fmt.Sprintf(`[Unit]
Description=Notebot local daemon
After=network.target

[Service]
ExecStart=%s
Restart=on-failure
RestartSec=5
Environment=NOTEBOT_DATA=%s
Environment=NOTEBOT_PORT=%d

[Install]
WantedBy=default.target
`, daemon, *dataDir, cfg.Port)
			if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
				return err
			}
			fmt.Println("wrote", path)

			if out, err := systemctl("daemon-reload"); err != nil {
				return fmt.Errorf("daemon-reload failed: %v: %s", err, out)
			}
			if noStart {
				fmt.Println("enable with: systemctl --user enable --now notebot")
				return nil
			}
			if out, err := systemctl("enable", "--now", unitName); err != nil {
				return fmt.Errorf("enable failed: %v: %s", err, out)
			}
			fmt.Println("notebot service enabled and started")
			return nil
		},
	}
	cmd.Flags().BoolVar(&noStart, "no-start", false, "install without enabling or starting")
	return cmd
}

func newUninstallServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-service",
		Short: "Stop and remove the systemd user service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := exec.LookPath("systemctl"); err != nil {
				return fmt.Errorf("systemctl not found; systemd is required")
			}
			path, err := unitPath()
			if err != nil {
				return err
			}
			if out, err := systemctl("disable", "--now", unitName); err != nil {
				fmt.Println("systemctl disable:", out)
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			if out, err := systemctl("daemon-reload"); err != nil {
				return fmt.Errorf("daemon-reload failed: %v: %s", err, out)
			}
			fmt.Println("removed", path)
			return nil
		},
	}
}
