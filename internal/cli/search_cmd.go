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
		opts.Keyword = cmd.Arg("keyword").String()
		opts.Extras = cmd.Arg("extras").Strings()

		if moreConditions := cmd.RemainArgs(); len(moreConditions) > 0 {
			opts.Extras = append(opts.Extras, moreConditions...)
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
	cmd.LongHelp = `<info>Examples</>:
  eget search markview
  eget search markview language:rust user:inhere
  eget search --limit 5 --sort stars --order desc terminal ui
  eget search --json picoclaw user:sipeed`

	cmd.StringVar(&opts.Sort, "sort", "", "Search sort field: stars, updated")
	cmd.StringVar(&opts.Order, "order", "", "Search order: desc, asc")
	cmd.IntVar(&opts.Limit, "limit", 10, "Limit result count;false;l")
	cmd.BoolVar(&opts.JSON, "json", false, "Output as JSON;false;j")

	cmd.AddArg("keyword", "keywords for search repositories", true, nil)
	cmd.AddArg("extras", "extra search conditions, allow multiple", false, nil)

	return cmd, func() {
		*opts = SearchOptions{Limit: 10}
	}
}
