package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type CacheTestSuite struct {
	suite.Suite
	entity params.GithubEntity
}

func (c *CacheTestSuite) SetupTest() {
	c.entity = params.GithubEntity{
		ID:         "1234",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
}

func (c *CacheTestSuite) TearDownTest() {
	// Clean up the cache after each test
	githubToolsCache.mux.Lock()
	defer githubToolsCache.mux.Unlock()
	githubToolsCache.entities = make(map[string]GithubEntityTools)
}

func (c *CacheTestSuite) TestCacheIsInitialized() {
	c.Require().NotNil(githubToolsCache)
}

func (c *CacheTestSuite) TestSetCacheWorks() {
	tools := []commonParams.RunnerApplicationDownload{
		{
			DownloadURL: garmTesting.Ptr("https://example.com"),
		},
	}

	c.Require().NotNil(githubToolsCache)
	c.Require().Len(githubToolsCache.entities, 0)
	SetGithubToolsCache(c.entity, tools)
	c.Require().Len(githubToolsCache.entities, 1)
	cachedTools, ok := GetGithubToolsCache(c.entity)
	c.Require().True(ok)
	c.Require().Len(cachedTools, 1)
	c.Require().Equal(tools[0].GetDownloadURL(), cachedTools[0].GetDownloadURL())
}

func (c *CacheTestSuite) TestTimedOutToolsCache() {
	tools := []commonParams.RunnerApplicationDownload{
		{
			DownloadURL: garmTesting.Ptr("https://example.com"),
		},
	}

	c.Require().NotNil(githubToolsCache)
	c.Require().Len(githubToolsCache.entities, 0)
	SetGithubToolsCache(c.entity, tools)
	c.Require().Len(githubToolsCache.entities, 1)
	entity := githubToolsCache.entities[c.entity.String()]
	entity.updatedAt = entity.updatedAt.Add(-2 * time.Hour)
	githubToolsCache.entities[c.entity.String()] = entity

	cachedTools, ok := GetGithubToolsCache(c.entity)
	c.Require().False(ok)
	c.Require().Nil(cachedTools)
}

func (c *CacheTestSuite) TestGetInexistentCache() {
	c.Require().NotNil(githubToolsCache)
	c.Require().Len(githubToolsCache.entities, 0)
	cachedTools, ok := GetGithubToolsCache(c.entity)
	c.Require().False(ok)
	c.Require().Nil(cachedTools)
}

func TestCacheTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CacheTestSuite))
}
