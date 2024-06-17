package watcher_test

import (
	"context"
	"testing"

	"github.com/cloudbase/garm/database"
	"github.com/cloudbase/garm/database/common"
	"github.com/cloudbase/garm/database/watcher"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/stretchr/testify/suite"
)

type WatcherStoreTestSuite struct {
	suite.Suite

	store common.Store
	ctx   context.Context
}

func (s *WatcherStoreTestSuite) TestGithubEndpointWatcher() {
	// ghEpParams := params.CreateGithubEndpointParams{
	// 	Name:          "test",
	// 	Description:   "test endpoint",
	// 	APIBaseURL:    "https://api.ghes.example.com",
	// 	UploadBaseURL: "https://upload.ghes.example.com",
	// 	BaseURL:       "https://ghes.example.com",
	// }

}

func TestWatcherStoreTestSuite(t *testing.T) {
	ctx := context.TODO()
	watcher.InitWatcher(ctx)

	store, err := database.NewDatabase(ctx, garmTesting.GetTestSqliteDBConfig(t))
	if err != nil {
		t.Fatalf("failed to create db connection: %s", err)
	}
	watcherSuite := &WatcherStoreTestSuite{
		ctx:   context.TODO(),
		store: store,
	}
	suite.Run(t, watcherSuite)
}
