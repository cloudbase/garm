package templates

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	"github.com/cloudbase/garm-provider-common/defaults"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm-provider-common/util"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/params"
)

var (
	poolIDLabelprefix     = "runner-pool-id"
	controllerLabelPrefix = "runner-controller-id"
)

//go:embed all:userdata
var Userdata embed.FS

func getLabelsForInstance(instance params.Instance) []string {
	jitEnabled := len(instance.JitConfiguration) > 0
	if jitEnabled {
		return []string{}
	}

	if instance.ScaleSetID > 0 {
		return []string{}
	}

	pool, ok := cache.GetPoolByID(instance.PoolID)
	if !ok {
		return []string{}
	}
	var labels []string
	for _, val := range pool.Tags {
		labels = append(labels, val.Name)
	}

	labels = append(labels, fmt.Sprintf("%s=%s", controllerLabelPrefix, cache.ControllerInfo().ControllerID.String()))
	labels = append(labels, fmt.Sprintf("%s=%s", poolIDLabelprefix, instance.PoolID))
	return labels
}

func RenderUserdata(instance params.Instance, entity params.ForgeEntity, token string) ([]byte, error) {
	tmpl, err := template.ParseFS(Userdata, "userdata/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	forgeType, err := entity.GetForgeType()
	if err != nil {
		return nil, fmt.Errorf("failed to get forge type: %w", err)
	}

	switch instance.OSType {
	case commonParams.Windows, commonParams.Linux:
	default:
		return nil, runnerErrors.NewBadRequestError("invalid OS type %q", instance.OSType)
	}

	tools, err := cache.GetGithubToolsCache(entity.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools: %w", err)
	}

	foundTools, err := util.GetTools(instance.OSType, instance.OSArch, tools)
	if err != nil {
		return nil, fmt.Errorf("failed to find tools: %w", err)
	}

	jitEnabled := len(instance.JitConfiguration) > 0

	installRunnerParams := cloudconfig.InstallRunnerParams{
		FileName:          foundTools.GetFilename(),
		DownloadURL:       foundTools.GetDownloadURL(),
		TempDownloadToken: foundTools.GetTempDownloadToken(),
		MetadataURL:       instance.MetadataURL,
		RunnerUsername:    defaults.DefaultUser,
		RunnerGroup:       defaults.DefaultUser,
		RepoURL:           entity.ForgeURL(),
		RunnerName:        instance.Name,
		RunnerLabels:      strings.Join(getLabelsForInstance(instance), ","),
		CallbackURL:       instance.CallbackURL,
		CallbackToken:     token,
		GitHubRunnerGroup: instance.GitHubRunnerGroup,
		UseJITConfig:      jitEnabled,
	}

	templateName := fmt.Sprintf("%s_%s_userdata.tmpl", forgeType, instance.OSType)

	var b bytes.Buffer
	wr := bufio.NewWriter(&b)
	wr.Flush()

	if err := tmpl.ExecuteTemplate(wr, templateName, installRunnerParams); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	wr.Flush()
	return b.Bytes(), nil
}
