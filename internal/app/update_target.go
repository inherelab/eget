package app

import (
	cfgpkg "github.com/inherelab/eget/internal/config"
	storepkg "github.com/inherelab/eget/internal/installed"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
)

func findUpdateTarget(cfg *cfgpkg.File, installed *storepkg.Config, target string) (ListItem, storepkg.Entry, bool, bool) {
	if cfg == nil {
		cfg = cfgpkg.NewFile()
	}
	if installed == nil {
		installed = &storepkg.Config{}
	}
	items, err := (ListService{
		LoadConfig: func() (*cfgpkg.File, error) {
			return cfg, nil
		},
		LoadInstalled: func() (*storepkg.Config, error) {
			return installed, nil
		},
	}).ListPackages()
	if err != nil {
		return ListItem{}, storepkg.Entry{}, false, false
	}

	normalizedTarget := storepkg.NormalizeRepoName(target)
	for _, item := range items {
		entry := installedEntryForItem(installed, item)
		if _, managed := cfg.Packages[item.Name]; managed && target == item.Name {
			return item, entry, true, true
		}
		if item.Name == target {
			_, managed := cfg.Packages[item.Name]
			return item, entry, managed, true
		}
		if item.Repo == target || item.Repo == normalizedTarget || entry.Target == target || storepkg.NormalizeRepoName(entry.Target) == normalizedTarget {
			_, managed := cfg.Packages[item.Name]
			return item, entry, managed, true
		}
	}
	return ListItem{}, storepkg.Entry{}, false, false
}

func installedEntryForItem(installed *storepkg.Config, item ListItem) storepkg.Entry {
	if installed == nil || installed.Installed == nil {
		return storepkg.Entry{}
	}
	if entry, ok := installed.Installed[item.Repo]; ok {
		return entry
	}
	normalized := storepkg.NormalizeRepoName(item.Repo)
	if entry, ok := installed.Installed[normalized]; ok {
		return entry
	}
	if entry, ok := installed.Installed[item.Name]; ok {
		return entry
	}
	return storepkg.Entry{}
}

func enrichListItemFromInstalledEntry(item *ListItem, entry storepkg.Entry) {
	if item == nil {
		return
	}
	if item.SourcePath == "" {
		if sourcePath, ok := stringOption(entry.Options, "source_path"); ok {
			item.SourcePath = sourcePath
		} else if sfTarget, err := sourcesf.ParseTarget(entry.Target); err == nil {
			item.SourcePath = sfTarget.Path
		}
	}
	if item.Tag == "" {
		if tag, ok := stringOption(entry.Options, "tag"); ok {
			item.Tag = trackingTag(tag)
		}
	} else {
		item.Tag = trackingTag(item.Tag)
	}
	if prerelease, ok := boolOption(entry.Options, "prerelease"); ok {
		item.Prerelease = prerelease
	}
}

func installedUpdateTarget(item ListItem, entry storepkg.Entry) string {
	if entry.Target != "" {
		return entry.Target
	}
	if item.Repo != "" {
		return item.Repo
	}
	return entry.Repo
}
