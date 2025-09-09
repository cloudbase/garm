// Copyright 2025 Cloudbase Solutions SRL
//
//	Licensed under the Apache License, Version 2.0 (the "License"); you may
//	not use this file except in compliance with the License. You may obtain
//	a copy of the License at
//
//	     http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//	WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//	License for the specific language governing permissions and limitations
//	under the License.
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
		entities:  make(map[string]EntityItem),
		pools:     make(map[string]params.Pool),
		scalesets: make(map[uint]params.ScaleSet),
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
		ID:    "pool-1",
		OrgID: "test-entity",
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
		ID:    "pool-1",
		OrgID: "test-entity",
	}
	pool2 := params.Pool{
		ID:    "pool-2",
		OrgID: "test-entity",
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
		ID:    1,
		OrgID: "test-entity",
	}
	scaleSet2 := params.ScaleSet{
		ID:    2,
		OrgID: "test-entity",
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
		OrgID: "test-entity",
		ID:    "pool-1",
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
		OrgID: "test-entity",
		ID:    1,
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
		ID:    "pool-1",
		OrgID: "test-entity",
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
		ID:    "pool-2",
		OrgID: "test-entity",
		Tags: []params.Tag{
			{
				Name: "tag1",
			},
		},
	}
	pool3 := params.Pool{
		ID:    "pool-3",
		OrgID: "test-entity",
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
		ID:    "pool-1",
		OrgID: "test-entity",
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
		ID:    "pool-2",
		OrgID: "test-entity",
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
		OrgID: "test-entity",
		ID:    1,
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
		ID:    "pool-1",
		OrgID: "test-entity",
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
		OrgID:     "test-entity",
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
		OrgID:     "test-entity",
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
		OrgID: "test-entity",
		ID:    1,
	}
	scaleSet2 := params.ScaleSet{
		OrgID: "test-entity",
		ID:    2,
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

func (c *CacheTestSuite) TestGetEntityForPool() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	pool := params.Pool{
		ID:    "pool-1",
		OrgID: "test-entity", // Set the org ID to match the entity
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool)

	retrievedEntity, ok := GetEntityForPool(pool.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, retrievedEntity.ID)
	c.Require().Equal(entity.Name, retrievedEntity.Name)

	nonExistentEntity, ok := GetEntityForPool("non-existent-pool")
	c.Require().False(ok)
	c.Require().Equal(params.ForgeEntity{}, nonExistentEntity)
}

func (c *CacheTestSuite) TestGetEntityForScaleSet() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	scaleSet := params.ScaleSet{
		ID:    1,
		OrgID: "test-entity", // Set the org ID to match the entity
	}

	SetEntity(entity)
	SetEntityScaleSet(entity.ID, scaleSet)

	retrievedEntity, ok := GetEntityForScaleSet(scaleSet.ID)
	c.Require().True(ok)
	c.Require().Equal(entity.ID, retrievedEntity.ID)
	c.Require().Equal(entity.Name, retrievedEntity.Name)

	nonExistentEntity, ok := GetEntityForScaleSet(999)
	c.Require().False(ok)
	c.Require().Equal(params.ForgeEntity{}, nonExistentEntity)
}

func (c *CacheTestSuite) TestGetPoolByID() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	pool := params.Pool{
		ID:      "pool-1",
		OrgID:   "test-entity",
		Enabled: true,
		Tags: []params.Tag{
			{
				Name: "test-tag",
			},
		},
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool)

	retrievedPool, ok := GetPoolByID(pool.ID)
	c.Require().True(ok)
	c.Require().Equal(pool.ID, retrievedPool.ID)
	c.Require().Equal(pool.Enabled, retrievedPool.Enabled)
	c.Require().Len(retrievedPool.Tags, 1)
	c.Require().Equal("test-tag", retrievedPool.Tags[0].Name)

	nonExistentPool, ok := GetPoolByID("non-existent-pool")
	c.Require().False(ok)
	c.Require().Equal(params.Pool{}, nonExistentPool)
}

func (c *CacheTestSuite) TestGetScaleSetByID() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	scaleSet := params.ScaleSet{
		ID:      1,
		OrgID:   "test-entity",
		Enabled: true,
	}

	SetEntity(entity)
	SetEntityScaleSet(entity.ID, scaleSet)

	retrievedScaleSet, ok := GetScaleSetByID(scaleSet.ID)
	c.Require().True(ok)
	c.Require().Equal(scaleSet.ID, retrievedScaleSet.ID)
	c.Require().Equal(scaleSet.Enabled, retrievedScaleSet.Enabled)

	nonExistentScaleSet, ok := GetScaleSetByID(999)
	c.Require().False(ok)
	c.Require().Equal(params.ScaleSet{}, nonExistentScaleSet)
}

func (c *CacheTestSuite) TestGetPoolByIDWithMultiplePools() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	pool1 := params.Pool{
		ID:      "pool-1",
		Enabled: true,
		OrgID:   "test-entity",
	}
	pool2 := params.Pool{
		ID:      "pool-2",
		Enabled: false,
		OrgID:   "test-entity",
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool1)
	SetEntityPool(entity.ID, pool2)

	retrievedPool1, ok := GetPoolByID("pool-1")
	c.Require().True(ok)
	c.Require().Equal("pool-1", retrievedPool1.ID)
	c.Require().True(retrievedPool1.Enabled)

	retrievedPool2, ok := GetPoolByID("pool-2")
	c.Require().True(ok)
	c.Require().Equal("pool-2", retrievedPool2.ID)
	c.Require().False(retrievedPool2.Enabled)
}

func (c *CacheTestSuite) TestGetScaleSetByIDWithMultipleScaleSets() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test",
		Owner:      "test",
	}
	scaleSet1 := params.ScaleSet{
		ID:      1,
		Enabled: true,
		OrgID:   "test-entity",
	}
	scaleSet2 := params.ScaleSet{
		ID:      2,
		Enabled: false,
		OrgID:   "test-entity",
	}

	SetEntity(entity)
	SetEntityScaleSet(entity.ID, scaleSet1)
	SetEntityScaleSet(entity.ID, scaleSet2)

	retrievedScaleSet1, ok := GetScaleSetByID(1)
	c.Require().True(ok)
	c.Require().Equal(uint(1), retrievedScaleSet1.ID)
	c.Require().True(retrievedScaleSet1.Enabled)

	retrievedScaleSet2, ok := GetScaleSetByID(2)
	c.Require().True(ok)
	c.Require().Equal(uint(2), retrievedScaleSet2.ID)
	c.Require().False(retrievedScaleSet2.Enabled)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithPool() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test-org",
		Owner:      "test-owner",
	}
	pool := params.Pool{
		ID:    "pool-1",
		OrgID: "test-entity",
	}
	instance := params.Instance{
		Name:   "test-instance",
		PoolID: "pool-1",
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool)
	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("test-instance")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, retrievedEntity.ID)
	c.Require().Equal(entity.Name, retrievedEntity.Name)
	c.Require().Equal(entity.Owner, retrievedEntity.Owner)
	c.Require().Equal(entity.EntityType, retrievedEntity.EntityType)

	nonExistentEntity, ok := GetEntityForInstance("non-existent-instance")
	c.Require().False(ok)
	c.Require().Equal(params.ForgeEntity{}, nonExistentEntity)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithScaleSet() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test-org",
		Owner:      "test-owner",
	}
	scaleSet := params.ScaleSet{
		ID:    1,
		OrgID: "test-entity",
	}
	instance := params.Instance{
		Name:       "test-instance",
		ScaleSetID: 1,
	}

	SetEntity(entity)
	SetEntityScaleSet(entity.ID, scaleSet)
	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("test-instance")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, retrievedEntity.ID)
	c.Require().Equal(entity.Name, retrievedEntity.Name)
	c.Require().Equal(entity.Owner, retrievedEntity.Owner)
	c.Require().Equal(entity.EntityType, retrievedEntity.EntityType)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithRepositoryEntity() {
	entity := params.ForgeEntity{
		ID:         "test-repo-entity",
		EntityType: params.ForgeEntityTypeRepository,
		Name:       "test-repo",
		Owner:      "test-owner",
	}
	pool := params.Pool{
		ID:     "pool-repo",
		RepoID: "test-repo-entity",
	}
	instance := params.Instance{
		Name:   "test-repo-instance",
		PoolID: "pool-repo",
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool)
	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("test-repo-instance")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, retrievedEntity.ID)
	c.Require().Equal(entity.EntityType, retrievedEntity.EntityType)
	c.Require().Equal("test-repo", retrievedEntity.Name)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithEnterpriseEntity() {
	entity := params.ForgeEntity{
		ID:         "test-enterprise-entity",
		EntityType: params.ForgeEntityTypeEnterprise,
		Name:       "test-enterprise",
		Owner:      "test-owner",
	}
	scaleSet := params.ScaleSet{
		ID:           2,
		EnterpriseID: "test-enterprise-entity",
	}
	instance := params.Instance{
		Name:       "test-enterprise-instance",
		ScaleSetID: 2,
	}

	SetEntity(entity)
	SetEntityScaleSet(entity.ID, scaleSet)
	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("test-enterprise-instance")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, retrievedEntity.ID)
	c.Require().Equal(entity.EntityType, retrievedEntity.EntityType)
	c.Require().Equal("test-enterprise", retrievedEntity.Name)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithNoPoolOrScaleSet() {
	instance := params.Instance{
		Name: "orphaned-instance",
		// No PoolID or ScaleSetID set
	}

	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("orphaned-instance")
	c.Require().False(ok)
	c.Require().Equal(params.ForgeEntity{}, retrievedEntity)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithNonExistentPool() {
	instance := params.Instance{
		Name:   "test-instance",
		PoolID: "non-existent-pool",
	}

	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("test-instance")
	c.Require().False(ok)
	c.Require().Equal(params.ForgeEntity{}, retrievedEntity)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithNonExistentScaleSet() {
	instance := params.Instance{
		Name:       "test-instance",
		ScaleSetID: 999,
	}

	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("test-instance")
	c.Require().False(ok)
	c.Require().Equal(params.ForgeEntity{}, retrievedEntity)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithNonExistentEntity() {
	pool := params.Pool{
		ID:    "pool-1",
		OrgID: "non-existent-entity",
	}
	instance := params.Instance{
		Name:   "test-instance",
		PoolID: "pool-1",
	}

	// Only set pool and instance, not the entity
	SetEntityPool("non-existent-entity", pool)
	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("test-instance")
	c.Require().False(ok)
	c.Require().Equal(params.ForgeEntity{}, retrievedEntity)
}

func (c *CacheTestSuite) TestGetEntityForInstanceWithBothPoolAndScaleSet() {
	entity := params.ForgeEntity{
		ID:         "test-entity",
		EntityType: params.ForgeEntityTypeOrganization,
		Name:       "test-org",
		Owner:      "test-owner",
	}
	pool := params.Pool{
		ID:    "pool-1",
		OrgID: "test-entity",
	}
	scaleSet := params.ScaleSet{
		ID:    1,
		OrgID: "test-entity",
	}
	// Instance with both pool and scale set - scale set should take precedence
	instance := params.Instance{
		Name:       "test-instance",
		PoolID:     "pool-1",
		ScaleSetID: 1,
	}

	SetEntity(entity)
	SetEntityPool(entity.ID, pool)
	SetEntityScaleSet(entity.ID, scaleSet)
	SetInstanceCache(instance)

	retrievedEntity, ok := GetEntityForInstance("test-instance")
	c.Require().True(ok)
	c.Require().Equal(entity.ID, retrievedEntity.ID)
	// Should retrieve entity via scale set (scale set takes precedence)
}

func TestCacheTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CacheTestSuite))
}
