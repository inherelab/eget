package cli

import "github.com/inherelab/eget/internal/app"

type cliService struct {
	appService       app.Service
	cfgService       app.ConfigService
	listService      app.ListService
	queryService     app.QueryService
	searchService    app.SearchService
	uninstallService app.UninstallService
	updService       app.UpdateService
}
