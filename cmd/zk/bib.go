package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var bibCmd = &cobra.Command{
	Use:   "bib",
	Short: "Manage bibliography and references",
	Long:  `Browse, add, and remove bibliographic references. Use [@key] in notes to cite them.`,
	Run: func(cmd *cobra.Command, args []string) {
		runBibList()
	},
}

var bibAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new reference",
	Run: func(cmd *cobra.Command, args []string) {
		runBibAdd()
	},
}

var bibListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all references with citation counts",
	Run: func(cmd *cobra.Command, args []string) {
		runBibList()
	},
}

var bibRemoveCmd = &cobra.Command{
	Use:   "remove [ref-id]",
	Short: "Remove a reference",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runBibRemove(args[0])
	},
}

var (
	bibType   string
	bibTitle  string
	bibAuthor string
	bibYear   string
	bibURL    string
	bibDesc   string
)

func init() {
	rootCmd.AddCommand(bibCmd)
	bibCmd.AddCommand(bibAddCmd)
	bibCmd.AddCommand(bibListCmd)
	bibCmd.AddCommand(bibRemoveCmd)

	bibAddCmd.Flags().StringVarP(&bibType, "type", "t", "book", "Type: book, movie, article, video, website")
	bibAddCmd.Flags().StringVarP(&bibTitle, "title", "T", "", "Title (required)")
	bibAddCmd.Flags().StringVarP(&bibAuthor, "author", "a", "", "Author/creator")
	bibAddCmd.Flags().StringVarP(&bibYear, "year", "y", "", "Year of publication")
	bibAddCmd.Flags().StringVarP(&bibURL, "url", "u", "", "URL")
	bibAddCmd.Flags().StringVarP(&bibDesc, "desc", "d", "", "Short description")
	bibAddCmd.MarkFlagRequired("title")
}

func openBibStore() (*store.Store, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(absRoot, ".zk", "index.db")
	return store.NewStore(dbPath)
}

func runBibAdd() {
	st, err := openBibStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	id := slugify(bibTitle)
	ref := &model.Ref{
		ID:          id,
		Type:        bibType,
		Title:       bibTitle,
		Author:      bibAuthor,
		Year:        bibYear,
		URL:         bibURL,
		Description: bibDesc,
	}

	if err := st.UpsertRef(ref); err != nil {
		fmt.Fprintf(os.Stderr, "Error adding reference: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Added reference: %s\n", ref.Title)
	fmt.Printf("Cite with: [@%s]\n", id)
}

func runBibList() {
	st, err := openBibStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	summaries, err := st.ListRefSummaries()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing references: %v\n", err)
		os.Exit(1)
	}

	if len(summaries) == 0 {
		fmt.Println("No references yet. Add one with: zk bib add -T \"Title\" -a \"Author\"")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTYPE\tTITLE\tAUTHOR\tYEAR\tCITED")
	fmt.Fprintln(w, "--\t----\t-----\t------\t----\t-----")
	for _, s := range summaries {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
			s.Ref.ID, s.Ref.Type, s.Ref.Title, s.Ref.Author, s.Ref.Year, s.Citations)
	}
	w.Flush()
}

func runBibRemove(id string) {
	st, err := openBibStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	ref, err := st.GetRef(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Reference '%s' not found.\n", id)
		os.Exit(1)
	}

	if err := st.DeleteRef(id); err != nil {
		fmt.Fprintf(os.Stderr, "Error removing reference: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed reference: %s (%s)\n", ref.Title, id)
}
