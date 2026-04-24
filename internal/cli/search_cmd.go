package cli

import "github.com/gookit/goutil/cflag/capp"

type SearchOptions struct {
	Keyword string
	Extras  []string
	Limit   int
	Sort    string
	Order   string
	JSON    bool
}

func newSearchCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &SearchOptions{Limit: 10}
	cmd := capp.NewCmd("search", "Search GitHub repositories", func(cmd *capp.Cmd) error {
		args := cmd.RemainArgs()
		if len(args) > 0 {
			opts.Keyword = args[0]
		}
		if len(args) > 1 {
			opts.Extras = append(opts.Extras[:0], args[1:]...)
		} else {
			opts.Extras = opts.Extras[:0]
		}

		snapshot := *opts
		if len(opts.Extras) > 0 {
			snapshot.Extras = append([]string(nil), opts.Extras...)
		}
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		return handler(cmd.Name, &snapshot)
	})

	cmd.StringVar(&opts.Sort, "sort", "", "Search sort field: stars, updated")
	cmd.StringVar(&opts.Order, "order", "", "Search order: desc, asc")
	cmd.IntVar(&opts.Limit, "limit", 10, "Limit result count;false;l")
	cmd.BoolVar(&opts.JSON, "json", false, "Output as JSON;false;j")
	return cmd, func() {
		*opts = SearchOptions{Limit: 10}
	}
}
