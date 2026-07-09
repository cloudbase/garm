package cache

import (
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

var templateCache = newKeyedCache[uint, params.Template](0)

func SetTemplateCache(tpl params.Template) {
	templateCache.Set(tpl.ID, tpl)
}

func GetTemplate(id uint) (params.Template, bool) {
	return templateCache.Get(id)
}

func ListTemplates(osType *commonParams.OSType, forgeType *params.EndpointType) []params.Template {
	return templateCache.List(func(tpl params.Template) bool {
		if osType != nil && tpl.OSType != *osType {
			return false
		}
		if forgeType != nil && tpl.ForgeType != *forgeType {
			return false
		}
		return true
	})
}

func DeleteTemplate(id uint) {
	templateCache.Delete(id)
}
