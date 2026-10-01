package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mhs003/notebot/internal/dates"
	"github.com/mhs003/notebot/internal/store"
	"github.com/spf13/cobra"
)

func addRangeFlags(cmd *cobra.Command) {
	cmd.Flags().String("since", "", "only notes on/after this time (2006-01-02, today, 3 days ago)")
	cmd.Flags().String("until", "", "only notes on/before this time")
	cmd.Flags().Bool("updated", false, "filter by updated time instead of created")
	cmd.Flags().Int("limit", 50, "maximum notes to show")
}

func rangeQuery(cmd *cobra.Command) (store.Query, error) {
	preset, _ := cmd.Flags().GetString("preset")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")
	byUpdated, _ := cmd.Flags().GetBool("updated")
	limit, _ := cmd.Flags().GetInt("limit")

	from, to, err := dates.Range(time.Now(), preset, since, until)
	if err != nil {
		return store.Query{}, err
	}
	return store.Query{Since: from, Until: to, ByUpdated: byUpdated, Limit: limit}, nil
}

func printNotes(notes []*store.Note) {
	if len(notes) == 0 {
		fmt.Println("no notes found")
		return
	}
	for _, n := range notes {
		fmt.Printf("%s  [%s] %s  (%s)\n", n.ID[:8], n.Folder, n.Title, n.Created.Local().Format("2006-01-02 15:04"))
	}
	fmt.Printf("\n%d note(s)\n", len(notes))
}

func newListCmd(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list [folder]",
		Aliases: []string{"ls"},
		Short:   "List notes, optionally filtered by folder and time",
		RunE: func(cmd *cobra.Command, args []string) error {
			q, err := rangeQuery(cmd)
			if err != nil {
				return err
			}
			folder := q.Folder
			if len(args) > 0 {
				folder = args[0]
			}
			q.Folder = folder
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			notes, err := st.Query(q)
			if err != nil {
				return err
			}
			printNotes(notes)
			return nil
		},
	}
	addRangeFlags(cmd)
	return cmd
}

func newFindCmd(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find <query...>",
		Short: "Search notes by text, optionally within a time range",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q, err := rangeQuery(cmd)
			if err != nil {
				return err
			}
			if folder, _ := cmd.Flags().GetString("folder"); folder != "" {
				q.Folder = folder
			}
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			notes, err := st.SearchQuery(strings.Join(args, " "), q)
			if err != nil {
				return err
			}
			printNotes(notes)
			return nil
		},
	}
	addRangeFlags(cmd)
	cmd.Flags().String("folder", "", "restrict to a folder")
	return cmd
}

func newPresetCmd(dataDir *string, name, preset, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			q, err := rangeQuery(cmd)
			if err != nil {
				return err
			}
			from, to, err := dates.Range(time.Now(), preset, "", "")
			if err != nil {
				return err
			}
			q.Since, q.Until = from, to
			st, _, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()
			notes, err := st.Query(q)
			if err != nil {
				return err
			}
			printNotes(notes)
			return nil
		},
	}
	cmd.Flags().Bool("updated", false, "filter by updated time instead of created")
	cmd.Flags().Int("limit", 50, "maximum notes to show")
	return cmd
}

func newShowCmd(dataDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Print a note in full",
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
			fmt.Printf("title:  %s\nfolder: %s\nid:     %s\ncreated: %s\nupdated: %s\n\n%s\n",
				n.Title, n.Folder, n.ID,
				n.Created.Local().Format("2006-01-02 15:04"),
				n.Updated.Local().Format("2006-01-02 15:04"),
				n.Body)
			return nil
		},
	}
}
