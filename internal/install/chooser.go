package install

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
)

type Chooser interface {
	Choose(name string, dir bool, mode fs.FileMode) (direct bool, possible bool)
}

type BinaryChooser struct {
	Tool string
}

type LiteralFileChooser struct {
	File string
}

type GlobChooser struct {
	expr string
	g    glob.Glob
	all  bool
}

type RegexChooser struct {
	expr string
	re   *regexp.Regexp
}

type MultiChooser struct {
	expr     string
	choosers []Chooser
}

type FilterChooser struct {
	expr        string
	include     []Chooser
	exclude     []Chooser
	implicitAll bool
}

func NewBinaryChooser(tool string) *BinaryChooser {
	return &BinaryChooser{Tool: tool}
}

func NewGlobChooser(gl string) (*GlobChooser, error) {
	g, err := glob.Compile(gl, '/')
	return &GlobChooser{g: g, expr: gl, all: gl == "*" || gl == "/"}, err
}

func newFilePatternChooser(expr string) (Chooser, error) {
	if !strings.HasPrefix(expr, "REG:") {
		return NewGlobChooser(expr)
	}

	expr = strings.TrimSpace(strings.TrimPrefix(expr, "REG:"))
	if expr == "" {
		return nil, fmt.Errorf("empty file regex expression")
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid file regex %q: %w", expr, err)
	}
	return &RegexChooser{expr: expr, re: re}, nil
}

func NewFileChooser(expr string) (Chooser, error) {
	parts := strings.Split(expr, ",")
	if len(parts) == 1 {
		part := strings.TrimSpace(expr)
		if strings.HasPrefix(part, "^") {
			return newFilterChooser([]string{part})
		}
		return newFilePatternChooser(part)
	}

	hasExclude := false
	rawParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		rawParts = append(rawParts, part)
		if strings.HasPrefix(part, "^") {
			hasExclude = true
		}
	}
	if hasExclude {
		return newFilterChooser(rawParts)
	}

	choosers := make([]Chooser, 0, len(parts))
	normalized := make([]string, 0, len(parts))
	for _, part := range rawParts {
		ch, err := newFilePatternChooser(part)
		if err != nil {
			return nil, err
		}
		choosers = append(choosers, ch)
		normalized = append(normalized, part)
	}
	if len(choosers) == 0 {
		return nil, fmt.Errorf("empty file chooser expression")
	}
	if len(choosers) == 1 {
		return choosers[0], nil
	}
	return &MultiChooser{
		expr:     strings.Join(normalized, ","),
		choosers: choosers,
	}, nil
}

func newFilterChooser(parts []string) (Chooser, error) {
	filter := &FilterChooser{}
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		anti := strings.HasPrefix(part, "^")
		expr := part
		if anti {
			expr = strings.TrimSpace(strings.TrimPrefix(part, "^"))
			if expr == "" {
				return nil, fmt.Errorf("empty file exclude expression")
			}
		}
		ch, err := newFilePatternChooser(expr)
		if err != nil {
			return nil, err
		}
		if anti {
			filter.exclude = append(filter.exclude, ch)
		} else {
			filter.include = append(filter.include, ch)
		}
		normalized = append(normalized, part)
	}
	if len(filter.include) == 0 {
		all, err := NewGlobChooser("*")
		if err != nil {
			return nil, err
		}
		filter.include = append(filter.include, all)
		filter.implicitAll = true
	}
	if len(filter.include) == 0 && len(filter.exclude) == 0 {
		return nil, fmt.Errorf("empty file chooser expression")
	}
	filter.expr = strings.Join(normalized, ",")
	return filter, nil
}

func (b *BinaryChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	if dir {
		return false, false
	}
	base := path.Base(archivePathForCompare(name))
	fmatch := base == b.Tool || base == b.Tool+".exe" || base == b.Tool+".appimage"
	possible := !mode.IsDir() && isExec(name, mode.Perm())
	return fmatch && possible, possible
}

func (b *BinaryChooser) String() string {
	return fmt.Sprintf("exe `%s`", b.Tool)
}

func (l *LiteralFileChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	name = archivePathForCompare(name)
	file := archivePathForCompare(l.File)
	return false, path.Base(name) == path.Base(file) && strings.HasSuffix(name, file)
}

func (l *LiteralFileChooser) String() string {
	return fmt.Sprintf("`%s`", l.File)
}

func (g *GlobChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	if g.all {
		return true, true
	}
	name = archivePathForCompare(name)
	if len(name) > 0 && name[len(name)-1] == '/' {
		name = name[:len(name)-1]
	}
	return false, g.g.Match(path.Base(name)) || g.g.Match(name)
}

func (g *GlobChooser) String() string {
	return fmt.Sprintf("`%s`", g.expr)
}

func (r *RegexChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	name = archivePathForCompare(name)
	return false, r.re.MatchString(path.Base(name)) || r.re.MatchString(name)
}

func (r *RegexChooser) String() string {
	return fmt.Sprintf("regex `%s`", r.expr)
}

func (m *MultiChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	for _, chooser := range m.choosers {
		direct, possible := chooser.Choose(name, dir, mode)
		if direct || possible {
			return direct, true
		}
	}
	return false, false
}

func (m *MultiChooser) String() string {
	return fmt.Sprintf("`%s`", m.expr)
}

func (f *FilterChooser) Choose(name string, dir bool, mode fs.FileMode) (bool, bool) {
	for _, chooser := range f.exclude {
		_, possible := chooser.Choose(name, dir, mode)
		if possible {
			return false, false
		}
	}
	for _, chooser := range f.include {
		direct, possible := chooser.Choose(name, dir, mode)
		if direct || possible {
			return direct && !f.implicitAll, true
		}
	}
	return false, false
}

func (f *FilterChooser) String() string {
	return fmt.Sprintf("`%s`", f.expr)
}
