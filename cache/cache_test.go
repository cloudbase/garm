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
	credentialsCache.cache = make(map[uint]params.GithubCredentials)
	instanceCache.cache = make(map[string]params.Instance)
	entityCache = &EntityCache{
		entities: make(map[string]EntityItem),
	}
}

func (c *CacheTestSuite) TestCacheIsInitialized() {
	c.Require().NotNil(githubToolsCache)
	c.Require().NotNil(credentialsCache)
	c.Require().NotNil(instanceCache)
	c.Require().NotNil(entityCache)
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

func (c *CacheTestSuite) TestSetGithubCredentials() {
	credentials := params.GithubCredentials{
		ID: 1,
	}
	SetGithubCredentials(credentials)
	cachedCreds, ok := GetGithubCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)
}

func (c *CacheTestSuite) TestGetGithubCredentials() {
	credentials := params.GithubCredentials{
		ID: 1,
	}
	SetGithubCredentials(credentials)
	cachedCreds, ok := GetGithubCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)

	nonExisting, ok := GetGithubCredentials(2)
	c.Require().False(ok)
	c.Require().Equal(params.GithubCredentials{}, nonExisting)
}

func (c *CacheTestSuite) TestDeleteGithubCredentials() {
	credentials := params.GithubCredentials{
		ID: 1,
	}
	SetGithubCredentials(credentials)
	cachedCreds, ok := GetGithubCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)

	DeleteGithubCredentials(1)
	cachedCreds, ok = GetGithubCredentials(1)
	c.Require().False(ok)
	c.Require().Equal(params.GithubCredentials{}, cachedCreds)
}

func (c *CacheTestSuite) TestGetAllGithubCredentials() {
	credentials1 := params.GithubCredentials{
		ID: 1,
	}
	credentials2 := params.GithubCredentials{
		ID: 2,
	}
	SetGithubCredentials(credentials1)
	SetGithubCredentials(credentials2)

	cachedCreds := GetAllGithubCredentials()
	c.Require().Len(cachedCreds, 2)
	c.Require().Contains(cachedCreds, credentials1)
	c.Require().Contains(cachedCreds, credentials2)
}

func (c *CacheTestSuite) TestSetInstanceCache() {
	instance := params.Instance{
		Name: "test-instance",
	}
	SetInstanceCache(instance)
	cachedInstance, ok := GetInstanceCache("test-instance")
	c.Require().True(ok)
	c.Require().Equal(instance.Name, cachedInstance.Name)
}

func (c *CacheTestSuite) TestGetInstanceCache() {
	instance := params.Instance{
		Name: "test-instance",
	}
	SetInstanceCache(instance)
	cachedInstance, ok := GetInstanceCache("test-instance")
	c.Require().True(ok)
	c.Require().Equal(instance.Name, cachedInstance.Name)

	nonExisting, ok := GetInstanceCache("non-existing")
	c.Require().False(ok)
	c.Require().Equal(params.Instance{}, nonExisting)
}

func (c *CacheTestSuite) TestDeleteInstanceCache() {
	instance := params.Instance{
		Name: "test-instance",
	}
	SetInstanceCache(instance)
	cachedInstance, ok := GetInstanceCache("test-instance")
	c.Require().True(ok)
	c.Require().Equal(instance.Name, cachedInstance.Name)

	DeleteInstanceCache("test-instance")
	cachedInstance, ok = GetInstanceCache("test-instance")
	c.Require().False(ok)
	c.Require().Equal(params.Instance{}, cachedInstance)
}

func (c *CacheTestSuite) TestGetAllInstances() {
	instance1 := params.Instance{
		Name: "test-instance-1",
	}
	instance2 := params.Instance{
		Name: "test-instance-2",
	}
	SetInstanceCache(instance1)
	SetInstanceCache(instance2)

	cachedInstances := GetAllInstancesCache()
	c.Require().Len(cachedInstances, 2)
	c.Require().Contains(cachedInstances, instance1)
	c.Require().Contains(cachedInstances, instance2)
}

func (c *CacheTestSuite) TestGetInstancesForPool() {
	instance1 := params.Instance{
		Name:   "test-instance-1",
		PoolID: "pool-1",
	}
	instance2 := params.Instance{
		Name:   "test-instance-2",
		PoolID: "pool-1",
	}
	instance3 := params.Instance{
		Name:   "test-instance-3",
		PoolID: "pool-2",
	}
	SetInstanceCache(instance1)
	SetInstanceCache(instance2)
	SetInstanceCache(instance3)

	cachedInstances := GetInstancesForPool("pool-1")
	c.Require().Len(cachedInstances, 2)
	c.Require().Contains(cachedInstances, instance1)
	c.Require().Contains(cachedInstances, instance2)

	cachedInstances = GetInstancesForPool("pool-2")
	c.Require().Len(cachedInstances, 1)
	c.Require().Contains(cachedInstances, instance3)
}

func (c *CacheTestSuite) TestGetInstancesForScaleSet() {
	instance1 := params.Instance{
		Name:       "test-instance-1",
		ScaleSetID: 1,
	}
	instance2 := params.Instance{
		Name:       "test-instance-2",
		ScaleSetID: 1,
	}
	instance3 := params.Instance{
		Name:       "test-instance-3",
		ScaleSetID: 2,
	}
	SetInstanceCache(instance1)
	SetInstanceCache(instance2)
	SetInstanceCache(instance3)

	cachedInstances := GetInstancesForScaleSet(1)
	c.Require().Len(cachedInstances, 2)
	c.Require().Contains(cachedInstances, instance1)
	c.Require().Contains(cachedInstances, instance2)

	cachedInstances = GetInstancesForScaleSet(2)
	c.Require().Len(cachedInstances, 1)
	c.Require().Contains(cachedInstances, instance3)
}

func (c *CacheTestSuite) TestSetGetEntityCache() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	SetEntity(entity)
	cachedEntity, ok := GetEntity("test-entity")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)

	entity.Credentials.Description = "test description"
	SetEntity(entity)
	cachedEntity, ok = GetEntity("test-entity")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	c.Require().Equal(entity.Credentials.Description, cachedEntity.Credentials.Description)
}

func (c *CacheTestSuite) TestReplaceEntityPools() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
		Credentials: params.GithubCredentials{
			ID: 1,
		},
	}
	pool1 := params.Pool{
		ID: "pool-1",
	}
	pool2 := params.Pool{
		ID: "pool-2",
	}

	credentials := params.GithubCredentials{
		ID:   1,
		Name: "test",
	}
	SetGithubCredentials(credentials)

	SetEntity(entity)
	ReplaceEntityPools(entity.ID, []params.Pool{pool1, pool2})
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	c.Require().Equal("test", cachedEntity.Credentials.Name)

	pools := GetEntityPools(entity.ID)
	c.Require().Len(pools, 2)
	c.Require().Contains(pools, pool1)
	c.Require().Contains(pools, pool2)
}

func (c *CacheTestSuite) TestReplaceEntityScaleSets() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	scaleSet1 := params.ScaleSet{
		ID: 1,
	}
	scaleSet2 := params.ScaleSet{
		ID: 2,
	}

	SetEntity(entity)
	ReplaceEntityScaleSets(entity.ID, map[uint]params.ScaleSet{1: scaleSet1, 2: scaleSet2})
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)

	scaleSets := GetEntityScaleSets(entity.ID)
	c.Require().Len(scaleSets, 2)
	c.Require().Contains(scaleSets, scaleSet1)
	c.Require().Contains(scaleSets, scaleSet2)
}

func (c *CacheTestSuite) TestDeleteEntity() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	SetEntity(entity)
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)

	DeleteEntity(entity.ID)
	cachedEntity, ok = GetEntity(entity.ID)
	c.Require().False(ok)
	c.Require().Equal(params.GithubEntity{}, cachedEntity)
}

func (c *CacheTestSuite) TestSetEntityPool() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	pool := params.Pool{
		ID: "pool-1",
	}

	SetEntity(entity)

	SetEntityPool(entity.ID, pool)
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	pools := GetEntityPools(entity.ID)
	c.Require().Len(pools, 1)
	c.Require().Contains(pools, pool)
	c.Require().False(pools[0].Enabled)

	pool.Enabled = true
	SetEntityPool(entity.ID, pool)
	cachedEntity, ok = GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	pools = GetEntityPools(entity.ID)
	c.Require().Len(pools, 1)
	c.Require().Contains(pools, pool)
	c.Require().True(pools[0].Enabled)
}

func (c *CacheTestSuite) TestSetEntityScaleSet() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	scaleSet := params.ScaleSet{
		ID: 1,
	}

	SetEntity(entity)
	SetEntityScaleSet(entity.ID, scaleSet)

	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	scaleSets := GetEntityScaleSets(entity.ID)
	c.Require().Len(scaleSets, 1)
	c.Require().Contains(scaleSets, scaleSet)
	c.Require().False(scaleSets[0].Enabled)

	scaleSet.Enabled = true
	SetEntityScaleSet(entity.ID, scaleSet)
	scaleSets = GetEntityScaleSets(entity.ID)
	c.Require().Len(scaleSets, 1)
	c.Require().Contains(scaleSets, scaleSet)
	c.Require().True(scaleSets[0].Enabled)
}

func (c *CacheTestSuite) TestDeleteEntityPool() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	pool := params.Pool{
		ID: "pool-1",
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool)
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)

	DeleteEntityPool(entity.ID, pool.ID)
	pools := GetEntityPools(entity.ID)
	c.Require().Len(pools, 0)
	c.Require().NotContains(pools, pool)
}

func (c *CacheTestSuite) TestDeleteEntityScaleSet() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	scaleSet := params.ScaleSet{
		ID: 1,
	}

	SetEntity(entity)
	SetEntityScaleSet(entity.ID, scaleSet)
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)

	DeleteEntityScaleSet(entity.ID, scaleSet.ID)
	scaleSets := GetEntityScaleSets(entity.ID)
	c.Require().Len(scaleSets, 0)
	c.Require().NotContains(scaleSets, scaleSet)
}

func (c *CacheTestSuite) TestFindPoolsMatchingAllTags() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	pool1 := params.Pool{
		ID: "pool-1",
		Tags: []params.Tag{
			{
				Name: "tag1",
			},
			{
				Name: "tag2",
			},
		},
	}
	pool2 := params.Pool{
		ID: "pool-2",
		Tags: []params.Tag{
			{
				Name: "tag1",
			},
		},
	}
	pool3 := params.Pool{
		ID: "pool-3",
		Tags: []params.Tag{
			{
				Name: "tag3",
			},
		},
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool1)
	SetEntityPool(entity.ID, pool2)
	SetEntityPool(entity.ID, pool3)

	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	pools := FindPoolsMatchingAllTags(entity.ID, []string{"tag1", "tag2"})
	c.Require().Len(pools, 1)
	c.Require().Contains(pools, pool1)
	pools = FindPoolsMatchingAllTags(entity.ID, []string{"tag1"})
	c.Require().Len(pools, 2)
	c.Require().Contains(pools, pool1)
	c.Require().Contains(pools, pool2)
	pools = FindPoolsMatchingAllTags(entity.ID, []string{"tag3"})
	c.Require().Len(pools, 1)
	c.Require().Contains(pools, pool3)
	pools = FindPoolsMatchingAllTags(entity.ID, []string{"tag4"})
	c.Require().Len(pools, 0)
}

func (c *CacheTestSuite) TestGetEntityPools() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	pool1 := params.Pool{
		ID: "pool-1",
		Tags: []params.Tag{
			{
				Name: "tag1",
			},
			{
				Name: "tag2",
			},
		},
	}
	pool2 := params.Pool{
		ID: "pool-2",
		Tags: []params.Tag{
			{
				Name: "tag1",
			},
			{
				Name: "tag3",
			},
		},
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool1)
	SetEntityPool(entity.ID, pool2)
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	pools := GetEntityPools(entity.ID)
	c.Require().Len(pools, 2)
	c.Require().Contains(pools, pool1)
	c.Require().Contains(pools, pool2)
}

func (c *CacheTestSuite) TestGetEntityScaleSet() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	scaleSet := params.ScaleSet{
		ID: 1,
	}

	SetEntity(entity)
	SetEntityScaleSet(entity.ID, scaleSet)

	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	scaleSets, ok := GetEntityScaleSet(entity.ID, scaleSet.ID)
	c.Require().True(ok)
	c.Require().Equal(scaleSet.ID, scaleSets.ID)
}

func (c *CacheTestSuite) TestGetEntityPool() {
	entity := params.GithubEntity{
		ID:         "test-entity",
		EntityType: params.GithubEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}

	pool := params.Pool{
		ID: "pool-1",
		Tags: []params.Tag{
			{
				Name: "tag1",
			},
			{
				Name: "tag2",
			},
		},
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool)
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	poolFromCache, ok := GetEntityPool(entity.ID, pool.ID)
	c.Require().True(ok)
	c.Require().Equal(pool.ID, poolFromCache.ID)
}

func TestCacheTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CacheTestSuite))
}
