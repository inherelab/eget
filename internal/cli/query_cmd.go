package cli

import "github.com/gookit/goutil/cflag/capp"

type QueryOptions struct {
	Action     string
	Tag        string
	Limit      int
	JSON       bool
	Prerelease bool
	Target     string
}

func newQueryCmd(handler CommandHandler) (*capp.Cmd, func()) {
	opts := &QueryOptions{Action: "latest", Limit: 10}
	cmd := capp.NewCmd("query", "Query GitHub repository release metadata", func(cmd *capp.Cmd) error {
		opts.Target = cmd.Arg("target").String()
		if err := validateNoTrailingFlags(cmd); err != nil {
			return err
		}
		snapshot := *opts
		return handler(cmd.Name, &snapshot)
	})

	cmd.Aliases = []string{"q"}
	cmd.LongHelp = `<info>Query actions</>:
  latest              Show latest release info (default)
  releases            List recent releases
  assets              List release assets
  info                Show repository metadata

<info>Examples</>:
  eget query owner/repo
  eget query --action info owner/repo
  eget query --action releases --limit 20 owner/repo
  eget query --action assets --tag v1.2.3 owner/repo
  eget query --action latest --json owner/repo`

	cmd.StringVar(&opts.Action, "action", "latest", "Query action: latest, releases, assets, info;false;a")
	cmd.StringVar(&opts.Tag, "tag", "", "Release tag for assets action;false;t")
	cmd.IntVar(&opts.Limit, "limit", 10, "Limit release count for releases action;false;l")
	cmd.BoolVar(&opts.JSON, "json", false, "Output as JSON;false;j")
	cmd.BoolVar(&opts.Prerelease, "prerelease", false, "Include prerelease entries;false;p")
	cmd.AddArg("target", "Repository target owner/repo", true, nil)
	return cmd, func() {
		*opts = QueryOptions{Action: "latest", Limit: 10}
	}
}
