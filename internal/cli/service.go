package cli

import (
	"context"
	"io"
	"os"

	"github.com/inherelab/eget/internal/app"
	appcache "github.com/inherelab/eget/internal/app/cache"
	"github.com/inherelab/eget/internal/extpkg"
	"github.com/inherelab/eget/internal/sdk"
)

type sdkCLIService interface {
	InstallMany(context.Context, []string, sdk.InstallOptions) ([]sdk.InstallResult, error)
	DownloadMany(context.Context, []string, sdk.SDKDownloadOptions) ([]sdk.SDKDownloadResult, error)
	List(string) ([]sdk.InstalledEntry, error)
	Remove(string) (sdk.RemoveResult, error)
	Path(string) (sdk.InstalledEntry, error)
	SearchIndex(string, sdk.SearchOptions) ([]sdk.SearchResult, error)
	RefreshIndex(context.Context, string) (sdk.Index, error)
	RefreshAllIndexes(context.Context) ([]sdk.Index, error)
	ShowIndex(string) (sdk.Index, error)
	ListIndexes() ([]sdk.CachedIndexInfo, error)
	ClearIndex(string) error
	ClearAllIndexes() error
}

type selfUpdateCLIService interface {
	Update(app.SelfUpdateOptions) (app.SelfUpdateResult, error)
}

type cliService struct {
	appService        app.Service
	cfgService        app.ConfigService
	listService       app.ListService
	showService       app.ShowService
	queryService      app.QueryService
	searchService     app.SearchService
	uninstallService  app.UninstallService
	updService        app.UpdateService
	selfUpdateService selfUpdateCLIService
	sdkService        sdkCLIService
	cacheService      appcache.Service
	// extService drives packages owned by external managers (npm, uv, ...).
	extService app.ExternalProvider
	// extPkg keeps the concrete ext manager service: the web console also needs
	// Path(), which is not part of the app-layer interface.
	extPkg extpkg.Service

	stderr             io.Writer
	configPathResolver func() (string, error)
	lookupEnv          func(string) (string, bool)
	lookupUserHome     func(string) (string, error)
	fileExists         func(string) bool
	proxyURL           string
	proxyExclude       []string
	noProxy            bool
}

func (s *cliService) stderrWriter() io.Writer {
	if s != nil && s.stderr != nil {
		return s.stderr
	}
	return os.Stderr
}
