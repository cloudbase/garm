package cache

import (
	"sort"

	"github.com/cloudbase/garm/params"
)

func sortByID[T params.IDGetter](s []T) {
	sort.Slice(s, func(i, j int) bool {
		return s[i].GetID() < s[j].GetID()
	})
}

func sortByCreationDate[T params.CreationDateGetter](s []T) {
	sort.Slice(s, func(i, j int) bool {
		return s[i].GetCreatedAt().Before(s[j].GetCreatedAt())
	})
}
