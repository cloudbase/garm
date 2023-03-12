// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

//go:build testing
// +build testing

package testing

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/cloudbase/garm/config"

	"github.com/stretchr/testify/require"
)

var (
	encryptionPassphrase = "bocyasicgatEtenOubwonIbsudNutDom"
)

func GetTestSqliteDBConfig(t *testing.T) config.Database {
	dir, err := os.MkdirTemp("", "garm-config-test")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %s", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	return config.Database{
		Debug:      false,
		DbBackend:  config.SQLiteBackend,
		Passphrase: encryptionPassphrase,
		SQLite: config.SQLite{
			DBFile: filepath.Join(dir, "garm.db"),
		},
	}
}

type IDDBEntity interface {
	GetID() string
}

type NameAndIDDBEntity interface {
	IDDBEntity
	GetName() string
}

func EqualDBEntityByName[T NameAndIDDBEntity](t *testing.T, expected, actual []T) {
	require.Equal(t, len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].GetName() > expected[j].GetName() })
	sort.Slice(actual, func(i, j int) bool { return actual[i].GetName() > actual[j].GetName() })

	for i := 0; i < len(expected); i++ {
		require.Equal(t, expected[i].GetName(), actual[i].GetName())
	}
}

func EqualDBEntityID[T IDDBEntity](t *testing.T, expected, actual []T) {
	require.Equal(t, len(expected), len(actual))

	sort.Slice(expected, func(i, j int) bool { return expected[i].GetID() > expected[j].GetID() })
	sort.Slice(actual, func(i, j int) bool { return actual[i].GetID() > actual[j].GetID() })

	for i := 0; i < len(expected); i++ {
		require.Equal(t, expected[i].GetID(), actual[i].GetID())
	}
}

func DBEntityMapToSlice[T NameAndIDDBEntity](orgs map[string]T) []T {
	orgsSlice := []T{}
	for _, value := range orgs {
		orgsSlice = append(orgsSlice, value)
	}
	return orgsSlice
}
