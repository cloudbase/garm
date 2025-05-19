package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	garmTesting "github.com/cloudbase/garm/internal/testing"
	"github.com/cloudbase/garm/params"
)

type CacheTestSuite struct {
	suite.Suite
	entity params.ForgeEntity
}

func (c *CacheTestSuite) SetupTest() {
	c.entity = params.ForgeEntity{
		ID:         "1234",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
		Credentials: params.ForgeCredentials{
			ID:        1,
			Name:      "test",
			ForgeType: params.GithubEndpointType,
		},
	}
}

func (c *CacheTestSuite) TearDownTest() {
	// Clean up the cache after each test
	githubToolsCache.mux.Lock()
	defer githubToolsCache.mux.Unlock()
	githubToolsCache.entities = make(map[string]GithubEntityTools)
	giteaCredentialsCache.cache = make(map[uint]params.ForgeCredentials)
	credentialsCache.cache = make(map[uint]params.ForgeCredentials)
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

func (c *CacheTestSuite) TestSetToolsCacheWorks() {
	tools := []commonParams.RunnerApplicationDownload{
		{
			DownloadURL: garmTesting.Ptr("https://example.com"),
		},
	}
	c.Require().NotNil(githubToolsCache)
	c.Require().Len(githubToolsCache.entities, 0)
	SetGithubToolsCache(c.entity, tools)
	c.Require().Len(githubToolsCache.entities, 1)
	cachedTools, err := GetGithubToolsCache(c.entity.ID)
	c.Require().NoError(err)
	c.Require().Len(cachedTools, 1)
	c.Require().Equal(tools[0].GetDownloadURL(), cachedTools[0].GetDownloadURL())
}

func (c *CacheTestSuite) TestSetToolsCacheWithError() {
	tools := []commonParams.RunnerApplicationDownload{
		{
			DownloadURL: garmTesting.Ptr("https://example.com"),
		},
	}
	c.Require().NotNil(githubToolsCache)
	c.Require().Len(githubToolsCache.entities, 0)
	SetGithubToolsCache(c.entity, tools)
	entity := githubToolsCache.entities[c.entity.ID]

	c.Require().Equal(int64(entity.expiresAt.Sub(entity.updatedAt).Minutes()), int64(60))
	c.Require().Len(githubToolsCache.entities, 1)
	SetGithubToolsCacheError(c.entity, runnerErrors.ErrNotFound)

	cachedTools, err := GetGithubToolsCache(c.entity.ID)
	c.Require().Error(err)
	c.Require().Nil(cachedTools)
}

func (c *CacheTestSuite) TestSetErrorOnNonExistingCacheEntity() {
	entity := params.ForgeEntity{
		ID: "non-existing-entity",
	}
	c.Require().NotNil(githubToolsCache)
	c.Require().Len(githubToolsCache.entities, 0)
	SetGithubToolsCacheError(entity, runnerErrors.ErrNotFound)

	storedEntity, err := GetGithubToolsCache(entity.ID)
	c.Require().Error(err)
	c.Require().Nil(storedEntity)
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
	entity := githubToolsCache.entities[c.entity.ID]

	c.Require().Equal(int64(entity.expiresAt.Sub(entity.updatedAt).Minutes()), int64(60))
	c.Require().Len(githubToolsCache.entities, 1)
	entity = githubToolsCache.entities[c.entity.ID]
	entity.updatedAt = entity.updatedAt.Add(-3 * time.Hour)
	entity.expiresAt = entity.updatedAt.Add(-2 * time.Hour)
	githubToolsCache.entities[c.entity.ID] = entity

	cachedTools, err := GetGithubToolsCache(c.entity.ID)
	c.Require().Error(err)
	c.Require().Nil(cachedTools)
}

func (c *CacheTestSuite) TestGetInexistentCache() {
	c.Require().NotNil(githubToolsCache)
	c.Require().Len(githubToolsCache.entities, 0)
	cachedTools, err := GetGithubToolsCache(c.entity.ID)
	c.Require().Error(err)
	c.Require().Nil(cachedTools)
}

func (c *CacheTestSuite) TestSetGithubCredentials() {
	credentials := params.ForgeCredentials{
		ID: 1,
	}
	SetGithubCredentials(credentials)
	cachedCreds, ok := GetGithubCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)
}

func (c *CacheTestSuite) TestGetGithubCredentials() {
	credentials := params.ForgeCredentials{
		ID: 1,
	}
	SetGithubCredentials(credentials)
	cachedCreds, ok := GetGithubCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)

	nonExisting, ok := GetGithubCredentials(2)
	c.Require().False(ok)
	c.Require().Equal(params.ForgeCredentials{}, nonExisting)
}

func (c *CacheTestSuite) TestDeleteGithubCredentials() {
	credentials := params.ForgeCredentials{
		ID: 1,
	}
	SetGithubCredentials(credentials)
	cachedCreds, ok := GetGithubCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)

	DeleteGithubCredentials(1)
	cachedCreds, ok = GetGithubCredentials(1)
	c.Require().False(ok)
	c.Require().Equal(params.ForgeCredentials{}, cachedCreds)
}

func (c *CacheTestSuite) TestGetAllGithubCredentials() {
	credentials1 := params.ForgeCredentials{
		ID: 1,
	}
	credentials2 := params.ForgeCredentials{
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	SetEntity(entity)
	cachedEntity, ok := GetEntity("test-entity")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)

	pool := params.Pool{
		ID: "pool-1",
	}
	SetEntityPool(entity.ID, pool)
	cachedEntityPools := GetEntityPools("test-entity")
	c.Require().Equal(1, len(cachedEntityPools))

	entity.Credentials.Description = "test description"
	SetEntity(entity)
	cachedEntity, ok = GetEntity("test-entity")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	c.Require().Equal(entity.Credentials.Description, cachedEntity.Credentials.Description)

	// Make sure we don't clobber pools after updating the entity
	cachedEntityPools = GetEntityPools("test-entity")
	c.Require().Equal(1, len(cachedEntityPools))
}

func (c *CacheTestSuite) TestReplaceEntityPools() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
		Credentials: params.ForgeCredentials{
			ID:        1,
			ForgeType: params.GithubEndpointType,
		},
	}
	pool1 := params.Pool{
		ID: "pool-1",
	}
	pool2 := params.Pool{
		ID: "pool-2",
	}

	credentials := params.ForgeCredentials{
		ID:        1,
		Name:      "test",
		ForgeType: params.GithubEndpointType,
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	ReplaceEntityScaleSets(entity.ID, []params.ScaleSet{scaleSet1, scaleSet2})
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)

	scaleSets := GetEntityScaleSets(entity.ID)
	c.Require().Len(scaleSets, 2)
	c.Require().Contains(scaleSets, scaleSet1)
	c.Require().Contains(scaleSets, scaleSet2)
}

func (c *CacheTestSuite) TestDeleteEntity() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	c.Require().Equal(params.ForgeEntity{}, cachedEntity)
}

func (c *CacheTestSuite) TestSetEntityPool() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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

func (c *CacheTestSuite) TestSetGiteaCredentials() {
	credentials := params.ForgeCredentials{
		ID:          1,
		Description: "test description",
	}
	SetGiteaCredentials(credentials)
	cachedCreds, ok := GetGiteaCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)

	cachedCreds.Description = "new description"
	SetGiteaCredentials(cachedCreds)
	cachedCreds, ok = GetGiteaCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)
	c.Require().Equal("new description", cachedCreds.Description)
}

func (c *CacheTestSuite) TestGetAllGiteaCredentials() {
	credentials1 := params.ForgeCredentials{
		ID: 1,
	}
	credentials2 := params.ForgeCredentials{
		ID: 2,
	}
	SetGiteaCredentials(credentials1)
	SetGiteaCredentials(credentials2)

	cachedCreds := GetAllGiteaCredentials()
	c.Require().Len(cachedCreds, 2)
	c.Require().Contains(cachedCreds, credentials1)
	c.Require().Contains(cachedCreds, credentials2)
}

func (c *CacheTestSuite) TestDeleteGiteaCredentials() {
	credentials := params.ForgeCredentials{
		ID: 1,
	}
	SetGiteaCredentials(credentials)
	cachedCreds, ok := GetGiteaCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)

	DeleteGiteaCredentials(1)
	cachedCreds, ok = GetGiteaCredentials(1)
	c.Require().False(ok)
	c.Require().Equal(params.ForgeCredentials{}, cachedCreds)
}

func (c *CacheTestSuite) TestDeleteGiteaCredentialsNotFound() {
	credentials := params.ForgeCredentials{
		ID: 1,
	}
	SetGiteaCredentials(credentials)
	cachedCreds, ok := GetGiteaCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)

	DeleteGiteaCredentials(2)
	cachedCreds, ok = GetGiteaCredentials(1)
	c.Require().True(ok)
	c.Require().Equal(credentials.ID, cachedCreds.ID)
}

func (c *CacheTestSuite) TestUpdateCredentialsInAffectedEntities() {
	credentials := params.ForgeCredentials{
		ID:          1,
		Description: "test description",
	}
	entity1 := params.ForgeEntity{
		ID:          "test-entity-1",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials,
	}

	entity2 := params.ForgeEntity{
		ID:          "test-entity-2",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials,
	}

	SetEntity(entity1)
	SetEntity(entity2)

	cachedEntity1, ok := GetEntity(entity1.ID)
	c.Require().True(ok)
	c.Require().Equal(entity1.ID, cachedEntity1.ID)
	cachedEntity2, ok := GetEntity(entity2.ID)
	c.Require().True(ok)
	c.Require().Equal(entity2.ID, cachedEntity2.ID)

	c.Require().Equal(credentials.ID, cachedEntity1.Credentials.ID)
	c.Require().Equal(credentials.ID, cachedEntity2.Credentials.ID)
	c.Require().Equal(credentials.Description, cachedEntity1.Credentials.Description)
	c.Require().Equal(credentials.Description, cachedEntity2.Credentials.Description)

	credentials.Description = "new description"
	SetGiteaCredentials(credentials)

	cachedEntity1, ok = GetEntity(entity1.ID)
	c.Require().True(ok)
	c.Require().Equal(entity1.ID, cachedEntity1.ID)
	cachedEntity2, ok = GetEntity(entity2.ID)
	c.Require().True(ok)
	c.Require().Equal(entity2.ID, cachedEntity2.ID)

	c.Require().Equal(credentials.ID, cachedEntity1.Credentials.ID)
	c.Require().Equal(credentials.ID, cachedEntity2.Credentials.ID)
	c.Require().Equal(credentials.Description, cachedEntity1.Credentials.Description)
	c.Require().Equal(credentials.Description, cachedEntity2.Credentials.Description)
}

func (c *CacheTestSuite) TestSetGiteaEntity() {
	credentials := params.ForgeCredentials{
		ID:          1,
		Description: "test description",
		ForgeType:   params.GiteaEndpointType,
	}
	entity := params.ForgeEntity{
		ID:          "test-entity",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials,
	}

	SetGiteaCredentials(credentials)
	SetEntity(entity)

	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	c.Require().Equal(credentials.ID, cachedEntity.Credentials.ID)
	c.Require().Equal(credentials.Description, cachedEntity.Credentials.Description)
	c.Require().Equal(credentials.ForgeType, cachedEntity.Credentials.ForgeType)
}

func (c *CacheTestSuite) TestGetEntitiesUsingCredentials() {
	credentials := params.ForgeCredentials{
		ID:          1,
		Description: "test description",
		Name:        "test",
		ForgeType:   params.GithubEndpointType,
	}

	credentials2 := params.ForgeCredentials{
		ID:          2,
		Description: "test description2",
		Name:        "test",
		ForgeType:   params.GiteaEndpointType,
	}

	entity1 := params.ForgeEntity{
		ID:          "test-entity-1",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials,
	}

	entity2 := params.ForgeEntity{
		ID:          "test-entity-2",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials,
	}
	entity3 := params.ForgeEntity{
		ID:          "test-entity-3",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials2,
	}

	SetEntity(entity1)
	SetEntity(entity2)
	SetEntity(entity3)

	cachedEntities := GetEntitiesUsingCredentials(credentials)
	c.Require().Len(cachedEntities, 2)
	c.Require().Contains(cachedEntities, entity1)
	c.Require().Contains(cachedEntities, entity2)

	cachedEntities = GetEntitiesUsingCredentials(credentials2)
	c.Require().Len(cachedEntities, 1)
	c.Require().Contains(cachedEntities, entity3)
}

func (c *CacheTestSuite) TestGetallEntities() {
	credentials := params.ForgeCredentials{
		ID:          1,
		Description: "test description",
		Name:        "test",
		ForgeType:   params.GithubEndpointType,
	}

	credentials2 := params.ForgeCredentials{
		ID:          2,
		Description: "test description2",
		Name:        "test",
		ForgeType:   params.GiteaEndpointType,
	}

	entity1 := params.ForgeEntity{
		ID:          "test-entity-1",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials,
		CreatedAt:   time.Now(),
	}

	entity2 := params.ForgeEntity{
		ID:          "test-entity-2",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials,
		CreatedAt:   time.Now().Add(1 * time.Second),
	}

	entity3 := params.ForgeEntity{
		ID:          "test-entity-3",
		EntityType:  params.ForgeEntityTypeOrganization,
		Name:        "test",
		Owner:       "test",
		Credentials: credentials2,
		CreatedAt:   time.Now().Add(2 * time.Second),
	}

	SetEntity(entity1)
	SetEntity(entity2)
	SetEntity(entity3)

	// Sorted by creation date
	cachedEntities := GetAllEntities()
	c.Require().Len(cachedEntities, 3)
	c.Require().Equal(cachedEntities[0], entity1)
	c.Require().Equal(cachedEntities[1], entity2)
	c.Require().Equal(cachedEntities[2], entity3)
}

func (c *CacheTestSuite) TestGetAllPools() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	pool1 := params.Pool{
		ID:        "pool-1",
		CreatedAt: time.Now(),
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
		ID:        "pool-2",
		CreatedAt: time.Now().Add(1 * time.Second),
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
	pools := GetAllPools()
	c.Require().Len(pools, 2)
	c.Require().Equal(pools[0].ID, pool1.ID)
	c.Require().Equal(pools[1].ID, pool2.ID)
}

func (c *CacheTestSuite) TestGetAllScaleSets() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
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
	SetEntityScaleSet(entity.ID, scaleSet1)
	SetEntityScaleSet(entity.ID, scaleSet2)
	cachedEntity, ok := GetEntity(entity.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, cachedEntity.ID)
	scaleSets := GetAllScaleSets()
	c.Require().Len(scaleSets, 2)
	c.Require().Equal(scaleSets[0].ID, scaleSet1.ID)
	c.Require().Equal(scaleSets[1].ID, scaleSet2.ID)
}

func (c *CacheTestSuite) TestGetAllGetAllGithubCredentialsAsMap() {
	credentials1 := params.ForgeCredentials{
		ID: 1,
	}
	credentials2 := params.ForgeCredentials{
		ID: 2,
	}
	SetGithubCredentials(credentials1)
	SetGithubCredentials(credentials2)

	cachedCreds := GetAllGithubCredentialsAsMap()
	c.Require().Len(cachedCreds, 2)
	c.Require().Contains(cachedCreds, credentials1.ID)
	c.Require().Contains(cachedCreds, credentials2.ID)
}

func (c *CacheTestSuite) TestGetAllGiteaCredentialsAsMap() {
	credentials1 := params.ForgeCredentials{
		ID:        1,
		CreatedAt: time.Now(),
	}
	credentials2 := params.ForgeCredentials{
		ID:        2,
		CreatedAt: time.Now().Add(1 * time.Second),
	}
	SetGiteaCredentials(credentials1)
	SetGiteaCredentials(credentials2)

	cachedCreds := GetAllGiteaCredentialsAsMap()
	c.Require().Len(cachedCreds, 2)
	c.Require().Contains(cachedCreds, credentials1.ID)
	c.Require().Contains(cachedCreds, credentials2.ID)
}

func TestCacheTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CacheTestSuite))
}
