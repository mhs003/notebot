package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mhs003/notebot/internal/agent"
	"github.com/mhs003/notebot/internal/cliutil"
	"github.com/mhs003/notebot/internal/store"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var knownCommands = map[string]bool{
	"list": true, "ls": true, "recent": true, "search": true, "find": true,
	"folders": true, "show": true, "today": true, "yesterday": true,
	"rm": true, "delete": true, "mv": true, "move": true, "edit": true,
	"mkfolder": true, "mkdir": true, "rmfolder": true, "rmdir": true,
	"rename": true, "ask": true,
	"setup": true, "config": true, "serve": true, "help": true, "completion": true,
}

func isCaptureArg(a string) bool {
	if strings.HasPrefix(a, "-") {
		return false
	}
	return !knownCommands[a]
}

func main() {
	dataDir := store.DefaultDataDir()
	if v := os.Getenv("NOTEBOT_DATA"); v != "" {
		dataDir = v
	}
	if len(os.Args) > 1 && isCaptureArg(os.Args[1]) {
		var dry bool
		var folder string
		var text []string
		args := os.Args[1:]
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--dry-run":
				dry = true
			case a == "--folder" && i+1 < len(args):
				i++
				folder = args[i]
			case strings.HasPrefix(a, "--folder="):
				folder = strings.TrimPrefix(a, "--folder=")
			case a == "--":
				text = append(text, args[i+1:]...)
				i = len(args)
			case strings.HasPrefix(a, "-"):
				fmt.Fprintf(os.Stderr, "unknown flag %s for capture (only --dry-run and --folder apply)\n", a)
				os.Exit(2)
			default:
				text = append(text, a)
			}
		}
		if len(text) == 0 {
			fmt.Fprintln(os.Stderr, "usage: nb [--dry-run] [--folder NAME] <text...>")
			os.Exit(2)
		}
		if err := capture(dataDir, strings.Join(text, " "), folder, dry); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		return
	}
	root := &cobra.Command{
		Use:   "nb [text...]",
		Short: "Capture a note, refined by Needle3",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			dry, _ := cmd.Flags().GetBool("dry-run")
			folder, _ := cmd.Flags().GetString("folder")
			return capture(dataDir, strings.Join(args, " "), folder, dry)
		},
	}
	root.Flags().Bool("dry-run", false, "show refined note without saving")
	root.Flags().String("folder", "", "force folder")

	root.AddCommand(newListCmd(&dataDir))
	root.AddCommand(newFindCmd(&dataDir))
	root.AddCommand(newShowCmd(&dataDir))
	root.AddCommand(newPresetCmd(&dataDir, "today", "today", "Notes created today"))
	root.AddCommand(newPresetCmd(&dataDir, "yesterday", "yesterday", "Notes created yesterday"))
	root.AddCommand(newRemoveCmd(&dataDir))
	root.AddCommand(newMoveCmd(&dataDir))
	root.AddCommand(newEditCmd(&dataDir))
	root.AddCommand(newMkFolderCmd(&dataDir))
	root.AddCommand(newRmFolderCmd(&dataDir))
	root.AddCommand(newRenameFolderCmd(&dataDir))
	root.AddCommand(newAskCmd(&dataDir))
	root.AddCommand(&cobra.Command{
		Use:   "recent",
		Short: "Show recent notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listNotes(dataDir, "")
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "search <query>",
		Short: "Search notes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return searchNotes(dataDir, strings.Join(args, " "))
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "folders",
		Short: "List folders",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listFolders(dataDir)
		},
	})
	root.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Init data dir, copy model, set password",
		RunE: func(cmd *cobra.Command, args []string) error {
			return setup(dataDir)
		},
	})
	cfgCmd := &cobra.Command{Use: "config", Short: "Get/set config"}
	cfgCmd.AddCommand(&cobra.Command{
		Use:   "get <key>",
		Args:  cobra.ExactArgs(1),
		Short: "Get config value",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := store.LoadConfig(dataDir)
			if err != nil {
				return err
			}
			switch args[0] {
			case "model_path":
				fmt.Println(cfg.ModelPath)
			case "port":
				fmt.Println(cfg.Port)
			case "data_dir":
				fmt.Println(cfg.DataDir)
			default:
				return fmt.Errorf("unknown key %s", args[0])
			}
			return nil
		},
	})
	cfgCmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Args:  cobra.ExactArgs(2),
		Short: "Set config value",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := store.LoadConfig(dataDir)
			if err != nil {
				return err
			}
			switch args[0] {
			case "model_path":
				cfg.ModelPath = args[1]
			case "port":
				p, err := strconv.Atoi(args[1])
				if err != nil {
					return fmt.Errorf("port must be a number")
				}
				cfg.Port = p
			default:
				return fmt.Errorf("unknown key %s", args[0])
			}
			return cfg.Save()
		},
	})
	root.AddCommand(cfgCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadStore(dataDir string) (*store.Store, store.Config, error) {
	cfg, err := store.LoadConfig(dataDir)
	if err != nil {
		return nil, cfg, err
	}
	cfg.DataDir = dataDir
	st, err := store.Open(dataDir)
	return st, cfg, err
}

func daemonURL(cfg store.Config) string {
	return fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
}

func capture(dataDir, raw, forceFolder string, dry bool) error {
	st, cfg, err := loadStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	if !dry && tryDaemonCapture(cfg, raw) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	ag, err := agent.New(ctx, cfg, st)
	if err != nil {
		return err
	}
	defer ag.Close()
	r, err := ag.RefineAndRoute(ctx, raw)
	if err != nil {
		return err
	}
	if forceFolder != "" {
		r.Folder = forceFolder
	}
	if dry {
		fmt.Printf("title: %s\nfolder: %s\ntags: %s\n\n%s\n", r.Title, r.Folder, strings.Join(r.Tags, ","), r.Body)
		return nil
	}
	n := &store.Note{Title: r.Title, Body: r.Body, Folder: r.Folder, Tags: r.Tags, Source: "cli", RawPrompt: raw}
	if err := st.Create(n); err != nil {
		return err
	}
	fmt.Printf("saved: [%s] %s\n", n.Folder, n.Title)
	return nil
}

func tryDaemonCapture(cfg store.Config, raw string) bool {
	body, _ := json.Marshal(map[string]string{"raw": raw})
	req, err := http.NewRequest(http.MethodPost, daemonURL(cfg)+"/api/capture", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.PasswordHash != "" {
		tok := store.SessionToken(cfg.SessionSecret)
		if tok == "" {
			return false
		}
		req.AddCookie(&http.Cookie{Name: "notebot_session", Value: tok})
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var n store.Note
	if err := json.NewDecoder(resp.Body).Decode(&n); err != nil {
		return false
	}
	fmt.Printf("saved (daemon): [%s] %s\n", n.Folder, n.Title)
	return true
}

func listNotes(dataDir, folder string) error {
	st, _, err := loadStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	notes, err := st.List(folder, 30)
	if err != nil {
		return err
	}
	for _, n := range notes {
		fmt.Printf("%s [%s] %s\n", n.ID[:8], n.Folder, n.Title)
	}
	return nil
}

func searchNotes(dataDir, q string) error {
	st, _, err := loadStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	notes, err := st.Search(q, 20)
	if err != nil {
		return err
	}
	for _, n := range notes {
		fmt.Printf("%s [%s] %s\n", n.ID[:8], n.Folder, n.Title)
	}
	return nil
}

func listFolders(dataDir string) error {
	st, _, err := loadStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	folders, err := st.Folders()
	if err != nil {
		return err
	}
	for _, f := range folders {
		fmt.Println(f)
	}
	return nil
}

func setup(dataDir string) error {
	cfg, err := store.LoadConfig(dataDir)
	if err != nil {
		return err
	}
	cfg.DataDir = dataDir
	if err := store.EnsureDirs(dataDir); err != nil {
		return err
	}
	dst := filepath.Join(dataDir, "models", "needle3.cact")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		for _, src := range []string{filepath.Join("models", "needle3.cact")} {
			if f, err := os.Open(src); err == nil {
				func() {
					defer f.Close()
					out, err := os.Create(dst)
					if err != nil {
						return
					}
					defer out.Close()
					_, _ = io.Copy(out, f)
				}()
				fmt.Println("copied model to", dst)
				break
			}
		}
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			fmt.Println("model not found: copy models/needle3.cact to", dst)
		}
	}
	pw := cliutil.ReadPassword("Set dashboard password (empty to skip): ")
	if pw != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		cfg.PasswordHash = string(hash)
	}
	if err := cfg.Save(); err != nil {
		return err
	}
	fmt.Println("setup done:", dataDir)
	return nil
}
