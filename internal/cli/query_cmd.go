package cli

import "github.com/gookit/gcli/v3"

type QueryOptions struct {
	Action     string
	Tag        string
	Limit      int
	JSON       bool
	Prerelease bool
	Target     string
}

func newQueryCmd(handler CommandHandler) (*gcli.Command, func()) {
	opts := &QueryOptions{Action: "latest", Limit: 10}
	cmd := gcli.NewCommand("query", "Query repository, source, or direct URL metadata")
	cmd.Aliases = []string{"q"}
	cmd.Help = `<info>Query actions</>:
  latest              Show latest release info (default)
  releases            List recent releases
  assets              List release assets
  info                Show repository metadata

<info>Examples</>:
  eget query owner/repo
  eget query sourceforge:project
  eget query sf:project/path
  eget query --action info owner/repo
  eget query --action releases --limit 20 owner/repo
  eget query --action releases --limit 20 sf:project/path
  eget query --action assets --tag v1.2.3 owner/repo
  eget query --action assets --tag 1.2.3 sf:project/path
  eget query --action latest --json owner/repo
	eget query https://example.com/tool.zip
	eget query --json https://example.com/tool.zip

Direct URLs show HTTP file metadata. SourceForge targets support latest, releases and assets actions.`

	cmd.Config = func(c *gcli.Command) {
		c.StrOpt(&opts.Action, "action", "a", "latest", "Query action: latest, releases, assets, info")
		c.StrOpt(&opts.Tag, "tag", "t", "", "Release tag for assets action")
		c.IntOpt(&opts.Limit, "limit", "l", 10, "Limit release count for releases action")
		c.BoolOpt(&opts.JSON, "json", "j", false, "Output as JSON")
		c.BoolOpt(&opts.Prerelease, "prerelease", "p", false, "Include prerelease entries")
		c.AddArg("target", "Repository, SourceForge, or direct HTTP(S) URL target", true)
	}
	cmd.Func = func(c *gcli.Command, args []string) error {
		opts.Target = c.Arg("target").String()
		if err := validateNoFlagArgs(append([]string{opts.Target}, args...)); err != nil {
			return err
		}
		snapshot := *opts
		return handler("query", &snapshot)
	}
	return cmd, func() {
		*opts = QueryOptions{Action: "latest", Limit: 10}
	}
}
