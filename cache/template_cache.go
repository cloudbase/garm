package cache

import (
	"sync"

	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

var templateCache *TemplateCache

func init() {
	tplCache := &TemplateCache{
		cache: make(map[uint]params.Template),
	}
	templateCache = tplCache
}

type TemplateCache struct {
	mux sync.Mutex

	cache map[uint]params.Template
}

func (t *TemplateCache) SetTemplateCache(tpl params.Template) {
	t.mux.Lock()
	defer t.mux.Unlock()

	t.cache[tpl.ID] = tpl
}

func (t *TemplateCache) GetTemplate(id uint) (params.Template, bool) {
	t.mux.Lock()
	defer t.mux.Unlock()

	tpl, ok := t.cache[id]
	if !ok {
		return params.Template{}, false
	}

	return tpl, true
}

func (t *TemplateCache) ListTemplates(osType *commonParams.OSType, forgeType *params.EndpointType) []params.Template {
	ret := []params.Template{}
	for _, val := range t.cache {
		if osType != nil && val.OSType != *osType {
			continue
		}
		if forgeType != nil && val.ForgeType != *forgeType {
			continue
		}
		ret = append(ret, val)
	}
	return ret
}

func (t *TemplateCache) DeleteTemplate(id uint) {
	t.mux.Lock()
	defer t.mux.Unlock()

	delete(t.cache, id)
}

func SetTemplateCache(tpl params.Template) {
	templateCache.SetTemplateCache(tpl)
}

func GetTemplate(id uint) (params.Template, bool) {
	return templateCache.GetTemplate(id)
}

func ListTemplates(osType *commonParams.OSType, forgeType *params.EndpointType) []params.Template {
	return templateCache.ListTemplates(osType, forgeType)
}

func DeleteTemplate(id uint) {
	templateCache.DeleteTemplate(id)
}
