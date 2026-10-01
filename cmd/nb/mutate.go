package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mhs003/notebot/internal/store"
	"github.com/spf13/cobra"
)

func confirm(prompt string, assumeYes bool) bool {
	if assumeYes {
		return true
	}
	fmt.Printf("%s [y/N] ", prompt)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y")
}

func newRemoveCmd(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rm <id>",
		Aliases: []string{"delete"},
		Short:   "Delete a note",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			id, err := st.ResolveID(args[0])
			if err != nil {
				return err
			}
			n, err := st.Get(id)
			if err != nil {
				return err
			}
			if !confirm(fmt.Sprintf("Delete %q (%s)?", n.Title, n.Folder), yes) {
				fmt.Println("aborted")
				return nil
			}
			if err := st.Delete(id); err != nil {
				return err
			}
			fmt.Printf("deleted %s\n", id[:8])
			return nil
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation")
	return cmd
}

func newMoveCmd(dataDir *string) *cobra.Command {
	return &cobra.Command{
		Use:     "mv <id> <folder>",
		Aliases: []string{"move"},
		Short:   "Move a note to a folder",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			id, err := st.ResolveID(args[0])
			if err != nil {
				return err
			}
			if err := st.Move(id, args[1]); err != nil {
				return err
			}
			fmt.Printf("moved %s -> %s\n", id[:8], args[1])
			return nil
		},
	}
}

func newEditCmd(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a note's title, folder, or body (opens $EDITOR if no flags)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			id, err := st.ResolveID(args[0])
			if err != nil {
				return err
			}
			n, err := st.Get(id)
			if err != nil {
				return err
			}

			title, _ := cmd.Flags().GetString("title")
			folder, _ := cmd.Flags().GetString("folder")
			body, _ := cmd.Flags().GetString("body")
			bodyChanged := cmd.Flags().Changed("body")

			if title == "" && folder == "" && !bodyChanged {
				edited, err := editInEditor(n)
				if err != nil {
					return err
				}
				title, folder, body = edited.Title, edited.Folder, edited.Body
				bodyChanged = true
			}

			if title != "" {
				n.Title = title
			}
			if folder != "" {
				n.Folder = folder
			}
			if bodyChanged {
				n.Body = body
			}
			if err := st.Update(n); err != nil {
				return err
			}
			fmt.Printf("updated %s\n", id[:8])
			return nil
		},
	}
	cmd.Flags().String("title", "", "new title")
	cmd.Flags().String("folder", "", "move to folder")
	cmd.Flags().StringP("body", "b", "", "new body")
	return cmd
}

func editInEditor(n *store.Note) (*store.Note, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	dir, err := os.MkdirTemp("", "notebot-edit")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "note.md")
	content := fmt.Sprintf("title: %s\nfolder: %s\n---\n%s\n", n.Title, n.Folder, n.Body)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return nil, err
	}

	c := exec.Command(editor, path)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseEdited(n, string(raw)), nil
}

func newMkFolderCmd(dataDir *string) *cobra.Command {
	return &cobra.Command{
		Use:     "mkfolder <name>",
		Aliases: []string{"mkdir"},
		Short:   "Create a folder",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			if err := st.CreateFolder(args[0]); err != nil {
				return err
			}
			fmt.Printf("created folder %s\n", args[0])
			return nil
		},
	}
}

func newRmFolderCmd(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "rmfolder <name>",
		Aliases: []string{"rmdir"},
		Short:   "Delete a folder (its notes move to inbox)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			notes, err := st.List(args[0], 0)
			if err != nil {
				return err
			}
			msg := fmt.Sprintf("Delete folder %q?", args[0])
			if len(notes) > 0 {
				msg = fmt.Sprintf("Delete folder %q? Its %d note(s) move to inbox.", args[0], len(notes))
			}
			if !confirm(msg, yes) {
				fmt.Println("aborted")
				return nil
			}
			if err := st.DeleteFolder(args[0]); err != nil {
				return err
			}
			fmt.Printf("deleted folder %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation")
	return cmd
}

func newRenameFolderCmd(dataDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old> <new>",
		Short: "Rename a folder",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			if err := st.RenameFolder(args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("renamed %s -> %s\n", args[0], args[1])
			return nil
		},
	}
}
