package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"

	"github.com/mhs003/notebot/internal/agent"
	"github.com/mhs003/notebot/internal/cliutil"
	"github.com/mhs003/notebot/internal/server"
	"github.com/mhs003/notebot/internal/store"
	"github.com/mhs003/notebot/web"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	var setPassword bool
	var dataDirFlag string
	flag.BoolVar(&setPassword, "set-password", false, "set dashboard password and exit")
	flag.StringVar(&dataDirFlag, "data-dir", "", "override data dir")
	flag.Parse()

	dataDir := store.DefaultDataDir()
	if dataDirFlag != "" {
		dataDir = dataDirFlag
	}
	if v := os.Getenv("NOTEBOT_DATA"); v != "" {
		dataDir = v
	}
	cfg, err := store.LoadConfig(dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	cfg.DataDir = dataDir
	if _, err := cfg.EnsureSessionSecret(); err != nil {
		fmt.Fprintln(os.Stderr, "session secret:", err)
		os.Exit(1)
	}
	if port := os.Getenv("NOTEBOT_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	if setPassword {
		pw := cliutil.ReadPassword("New password: ")
		hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cfg.PasswordHash = string(hash)
		if err := cfg.Save(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("password set")
		return
	}

	st, err := store.Open(dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "store:", err)
		os.Exit(1)
	}
	defer st.Close()

	sub, err := fs.Sub(web.Dist, "dist")
	if err == nil {
		if _, err := sub.Open("index.html"); err == nil {
			server.WebFS = sub
		}
	}

	ctx := context.Background()
	ag, err := agent.New(ctx, cfg, st)
	if err != nil {
		fmt.Fprintln(os.Stderr, "agent:", err)
		os.Exit(1)
	}
	defer ag.Close()

	srv := server.New(cfg, st)
	srv.AttachAgentRoutes(ag)

	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	fmt.Println("notebotd listening on", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
