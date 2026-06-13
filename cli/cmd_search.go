package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// searchCmd returns the `search <query>` command.
func (a *App) searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search CSS-Tricks articles by title, summary, or category",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			n := a.effectiveLimit(20)
			a.progressf("searching articles for %q...", query)
			articles, err := a.client.Search(cmd.Context(), query, n)
			if err != nil {
				return mapFetchErr(err)
			}
			if len(articles) == 0 {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "no articles matched %q\n", query)
				return codeError(exitNoData, nil)
			}
			return a.renderOrEmpty(articles, len(articles))
		},
	}
}
