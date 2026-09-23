package sdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	cfgpkg "github.com/inherelab/eget/internal/config"
	"github.com/inherelab/eget/internal/util"
	"github.com/inherelab/eget/internal/util/atomicfile"
)

type Store struct {
	Path string
}

// storeMu serializes read-modify-write cycles on sdk.installed.json. The web
// console serves requests concurrently, so an unlocked load-then-save would
// lose entries.
var storeMu sync.Mutex

func DefaultStorePath() (string, error) {
	home, err := util.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(cfgpkg.OSConfigPath(home, "", os.LookupEnv)), "sdk.installed.json"), nil
}

func (s Store) Load() (InstalledStore, error) {
	storeMu.Lock()
	defer storeMu.Unlock()
	return s.load()
}

func (s Store) load() (InstalledStore, error) {
	if s.Path == "" {
		return newInstalledStore(), nil
	}
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return newInstalledStore(), nil
	}
	if err != nil {
		return InstalledStore{}, err
	}
	store := newInstalledStore()
	if err := json.Unmarshal(data, &store); err != nil {
		return InstalledStore{}, err
	}
	normalizeInstalledStore(&store)
	return store, nil
}

func (s Store) Save(store InstalledStore) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	return s.save(store)
}

func (s Store) save(store InstalledStore) error {
	normalizeInstalledStore(&store)
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(s.Path, append(data, '\n'), 0o644)
}

func (s Store) Record(entry InstalledEntry) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := s.load()
	if err != nil {
		return err
	}
	node := store.Installed[entry.Name]
	if node.Versions == nil {
		node.Versions = map[string]InstalledEntry{}
	}
	node.Versions[entry.Version] = entry
	store.Installed[entry.Name] = node
	return s.save(store)
}

func (s Store) Remove(name, version string) (InstalledEntry, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := s.load()
	if err != nil {
		return InstalledEntry{}, err
	}
	node, ok := store.Installed[name]
	if !ok {
		return InstalledEntry{}, fmt.Errorf("sdk %s is not installed", name)
	}
	entry, ok := node.Versions[version]
	if !ok {
		return InstalledEntry{}, fmt.Errorf("sdk %s@%s is not installed", name, version)
	}
	delete(node.Versions, version)
	if len(node.Versions) == 0 {
		delete(store.Installed, name)
	} else {
		store.Installed[name] = node
	}
	if err := s.save(store); err != nil {
		return InstalledEntry{}, err
	}
	return entry, nil
}

func (s Store) List(name string) ([]InstalledEntry, error) {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := s.load()
	if err != nil {
		return nil, err
	}
	var entries []InstalledEntry
	for sdkName, node := range store.Installed {
		if name != "" && sdkName != name {
			continue
		}
		for _, entry := range node.Versions {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name == entries[j].Name {
			return compareVersion(entries[i].Version, entries[j].Version) < 0
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func newInstalledStore() InstalledStore {
	return InstalledStore{
		Schema:    1,
		Installed: map[string]InstalledSDKNode{},
	}
}

func normalizeInstalledStore(store *InstalledStore) {
	if store.Schema == 0 {
		store.Schema = 1
	}
	if store.Installed == nil {
		store.Installed = map[string]InstalledSDKNode{}
	}
	for name, node := range store.Installed {
		if node.Versions == nil {
			node.Versions = map[string]InstalledEntry{}
			store.Installed[name] = node
		}
	}
}
