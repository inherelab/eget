package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gookit/goutil/x/assert"
	"github.com/inherelab/eget/internal/app"
	"github.com/inherelab/eget/internal/install"
	storepkg "github.com/inherelab/eget/internal/installed"
	"github.com/inherelab/eget/internal/sdk"
	forge "github.com/inherelab/eget/internal/source/forge"
	sourcegithub "github.com/inherelab/eget/internal/source/github"
	sourcesf "github.com/inherelab/eget/internal/source/sourceforge"
)

type fakeSDKService struct {
	installTargets  []string
	installOpts     sdk.InstallOptions
	installResults  []sdk.InstallResult
	downloadTargets []string
	downloadOpts    sdk.SDKDownloadOptions
	downloadResults []sdk.SDKDownloadResult
	listName        string
	listEntries     []sdk.InstalledEntry
	removeTarget    string
	removeResult    sdk.RemoveResult
	pathTarget      string
	pathEntry       sdk.InstalledEntry
	indexName       string
	indexAll        bool
	index           sdk.Index
	indexes         []sdk.Index
	cachedIndexes   []sdk.CachedIndexInfo
	searchName      string
	searchKeywords  []string
	searchNumber    int
	searchSort      string
	searchResults   []sdk.SearchResult
	clearName       string
	clearAll        bool
	err             error
}

type fakeUninstallStoreForCLI struct {
	cfg        *storepkg.Config
	removeKeys []string
}

func writeCLIFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func (f *fakeUninstallStoreForCLI) Load() (*storepkg.Config, error) {
	return f.cfg, nil
}

func (f *fakeUninstallStoreForCLI) Remove(target string) error {
	f.removeKeys = append(f.removeKeys, target)
	return nil
}

func (f *fakeSDKService) InstallMany(_ context.Context, targets []string, opts sdk.InstallOptions) ([]sdk.InstallResult, error) {
	f.installTargets = append([]string(nil), targets...)
	f.installOpts = opts
	for _, target := range targets {
		if opts.OnStart != nil {
			opts.OnStart(target, "1.21.1", "example.com")
		}
		if opts.OnArchiveReady != nil {
			opts.OnArchiveReady(sdk.DownloadResult{Path: "/cache/go.zip", FromCache: true})
		}
		if opts.OnExtractStart != nil {
			opts.OnExtractStart("/cache/go.zip", "/sdks/go1.21.1")
		}
	}
	return f.installResults, f.err
}

func (f *fakeSDKService) DownloadMany(_ context.Context, targets []string, opts sdk.SDKDownloadOptions) ([]sdk.SDKDownloadResult, error) {
	f.downloadTargets = append([]string(nil), targets...)
	f.downloadOpts = opts
	for _, target := range targets {
		if opts.OnStart != nil {
			opts.OnStart(target, "1.21.1", "example.com", opts.Platform.OS, opts.Platform.Arch)
		}
	}
	return f.downloadResults, f.err
}

func (f *fakeSDKService) List(name string) ([]sdk.InstalledEntry, error) {
	f.listName = name
	return f.listEntries, f.err
}

func (f *fakeSDKService) Remove(target string) (sdk.RemoveResult, error) {
	f.removeTarget = target
	return f.removeResult, f.err
}

func (f *fakeSDKService) Path(target string) (sdk.InstalledEntry, error) {
	f.pathTarget = target
	return f.pathEntry, f.err
}

func (f *fakeSDKService) RefreshIndex(_ context.Context, name string) (sdk.Index, error) {
	f.indexName = name
	return f.index, f.err
}

func (f *fakeSDKService) RefreshAllIndexes(_ context.Context) ([]sdk.Index, error) {
	f.indexAll = true
	return f.indexes, f.err
}

func (f *fakeSDKService) ShowIndex(name string) (sdk.Index, error) {
	f.indexName = name
	return f.index, f.err
}

func (f *fakeSDKService) ListIndexes() ([]sdk.CachedIndexInfo, error) {
	return f.cachedIndexes, f.err
}

func (f *fakeSDKService) SearchIndex(name string, opts sdk.SearchOptions) ([]sdk.SearchResult, error) {
	f.searchName = name
	f.searchKeywords = append([]string(nil), opts.Keywords...)
	f.searchNumber = opts.Number
	f.searchSort = opts.Sort
	return f.searchResults, f.err
}

func (f *fakeSDKService) ClearIndex(name string) error {
	f.clearName = name
	return f.err
}

func (f *fakeSDKService) ClearAllIndexes() error {
	f.clearAll = true
	return f.err
}

func TestNewCLIServiceWiresReleaseInfo(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))

	svc, err := newCLIService()
	if err != nil {
		t.Fatalf("newCLIService: %v", err)
	}
	if svc.appService.ReleaseInfo == nil {
		t.Fatal("expected ReleaseInfo to be configured")
	}
	sdkService, ok := svc.sdkService.(sdk.Service)
	if !ok {
		t.Fatalf("expected sdk.Service, got %T", svc.sdkService)
	}
	if sdkService.Config == nil {
		t.Fatal("expected sdk service config to be configured")
	}
	if sdkService.Store.Path == "" {
		t.Fatal("expected sdk installed store path to be configured")
	}
	if sdkService.IndexCache.Dir == "" {
		t.Fatal("expected sdk index cache dir to be configured")
	}
}

func TestNewCLIServiceLoadsDotenvBeforeConfig(t *testing.T) {
	t.Setenv("NO_PROXY", "")
	tmp := t.TempDir()
	xdgHome := filepath.Join(tmp, ".config")
	configDir := filepath.Join(xdgHome, "eget")
	writeCLIFile(t, filepath.Join(configDir, ".env"), "PROXY_URL=http://127.0.0.1:7890\nEGET_SELF_UPDATE_SOURCE=https://example.com/tools/eget/\n")
	writeCLIFile(t, filepath.Join(configDir, "eget.toml"), `
[global]
proxy_url = "${PROXY_URL}"
`)
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("XDG_CONFIG_HOME", xdgHome)
	t.Setenv("PROXY_URL", "")
	t.Setenv("EGET_SELF_UPDATE_SOURCE", "")

	svc, err := newCLIService()

	assert.NoErr(t, err)
	assert.Eq(t, "http://127.0.0.1:7890", svc.proxyURL)
	assert.Eq(t, "https://example.com/tools/eget/", os.Getenv("EGET_SELF_UPDATE_SOURCE"))
}

func TestNewCLIServiceIndexesInstalledAssetByRepoAndTarget(t *testing.T) {
	tmp := t.TempDir()
	xdgHome := filepath.Join(tmp, ".config")
	configDir := filepath.Join(xdgHome, "eget")
	writeCLIFile(t, filepath.Join(configDir, "installed.toml"), `
[installed.dbx]
repo = "t8y2/dbx"
target = "t8y2/dbx"
asset = "DBX_0.5.42_x64-setup.exe"
url = "https://github.com/t8y2/dbx/releases/download/v0.5.42/DBX_0.5.42_x64-setup.exe"
`)
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	svc, err := newCLIService()
	assert.NoErr(t, err)
	runner, ok := svc.appService.Runner.(*install.InstallRunner)
	assert.True(t, ok)

	assets, urls, err := runner.InstalledLoad()
	assert.NoErr(t, err)
	assert.Eq(t, "DBX_0.5.42_x64-setup.exe", assets["dbx"])
	assert.Eq(t, "DBX_0.5.42_x64-setup.exe", assets["t8y2/dbx"])
	assert.Eq(t, "https://github.com/t8y2/dbx/releases/download/v0.5.42/DBX_0.5.42_x64-setup.exe", urls["t8y2/dbx"])
}

func TestConfigureVerboseUpdatesVerboseLoggers(t *testing.T) {
	var out bytes.Buffer
	configureVerbose(true, &out)
	if !install.VerboseEnabledForTest() {
		t.Fatalf("expected install verbose to be enabled")
	}
	if !sourcegithub.VerboseEnabledForTest() {
		t.Fatalf("expected source verbose to be enabled")
	}
	if !sourcesf.VerboseEnabledForTest() {
		t.Fatalf("expected sourceforge verbose to be enabled")
	}
	if !forge.VerboseEnabledForTest() {
		t.Fatalf("expected forge verbose to be enabled")
	}
	configureVerbose(false, &out)
}

type fakeRunnerForCLI struct {
	result       app.RunResult
	targets      []string
	optsByTarget map[string]install.Options
}

func (f *fakeRunnerForCLI) Run(target string, opts install.Options) (app.RunResult, error) {
	f.targets = append(f.targets, target)
	if f.optsByTarget == nil {
		f.optsByTarget = map[string]install.Options{}
	}
	f.optsByTarget[target] = opts
	return f.result, nil
}

type fakeUpdateInstallerForCLI struct {
	targets     []string
	options     []install.Options
	errByTarget map[string]error
}

func (f *fakeUpdateInstallerForCLI) InstallTarget(target string, opts install.Options, extras ...app.InstallExtras) (app.RunResult, error) {
	f.targets = append(f.targets, target)
	f.options = append(f.options, opts)
	if f.errByTarget != nil {
		if err := f.errByTarget[target]; err != nil {
			return app.RunResult{}, err
		}
	}
	return app.RunResult{}, nil
}

type fakeSelfUpdateCLIService struct {
	opts   app.SelfUpdateOptions
	result app.SelfUpdateResult
}

func (f *fakeSelfUpdateCLIService) Update(opts app.SelfUpdateOptions) (app.SelfUpdateResult, error) {
	f.opts = opts
	return f.result, nil
}

type fakeInstalledStoreForCLI struct{}

func (f *fakeInstalledStoreForCLI) Record(target string, entry storepkg.Entry) error {
	return nil
}

type fakeConfigRecorderForCLI struct{}

func (f *fakeConfigRecorderForCLI) AddPackage(repo, name string, opts install.Options) error {
	return nil
}

func cliStringPtr(value string) *string {
	return &value
}

type fakeQueryClientForCLI struct {
	repoInfo QueryRepoInfoAlias
	releases []app.QueryRelease
	assets   []app.QueryAsset
}

type QueryRepoInfoAlias = app.QueryRepoInfo

func (f *fakeQueryClientForCLI) RepoInfo(repo string) (app.QueryRepoInfo, error) {
	info := app.QueryRepoInfo(f.repoInfo)
	if info.Repo == "" {
		info.Repo = repo
	}
	return info, nil
}

func (f *fakeQueryClientForCLI) LatestRelease(repo string, includePrerelease bool) (app.QueryRelease, error) {
	if len(f.releases) == 0 {
		return app.QueryRelease{}, nil
	}
	return f.releases[0], nil
}

func (f *fakeQueryClientForCLI) ListReleases(repo string, limit int, includePrerelease bool) ([]app.QueryRelease, error) {
	return f.releases, nil
}

func (f *fakeQueryClientForCLI) ReleaseAssets(repo, tag string) ([]app.QueryAsset, error) {
	return f.assets, nil
}

type fakeSearchClientForCLI struct {
	result app.SearchResult
	err    error
	query  string
	limit  int
	sort   string
	order  string
}

func (f *fakeSearchClientForCLI) SearchRepositories(query string, limit int, sort, order string) (app.SearchResult, error) {
	f.query = query
	f.limit = limit
	f.sort = sort
	f.order = order
	if f.err != nil {
		return app.SearchResult{}, f.err
	}
	return f.result, nil
}
