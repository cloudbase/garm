package external

import (
	"fmt"
	"garm/params"
	"strings"
)

const (
	envPrefix = "GARM"

	createInstanceCommand     = "GARM_COMMAND=CreateInstance"
	deleteInstanceCommand     = "GARM_COMMAND=DeleteInstance"
	getInstanceCommand        = "GARM_COMMAND=GetInstance"
	listInstancesCommand      = "GARM_COMMAND=ListInstances"
	startInstanceCommand      = "GARM_COMMAND=StartInstance"
	stopInstanceCommand       = "GARM_COMMAND=StopInstance"
	removeAllInstancesCommand = "GARM_COMMAND=RemoveAllInstances"
)

func bootstrapParamsToEnv(param params.BootstrapInstance) []string {
	ret := []string{
		fmt.Sprintf("%s_BOOTSTRAP_NAME='%s'", envPrefix, param.Name),
		fmt.Sprintf("%s_BOOTSTRAP_OS_ARCH='%s'", envPrefix, param.OSArch),
		fmt.Sprintf("%s_BOOTSTRAP_FLAVOR='%s'", envPrefix, param.Flavor),
		fmt.Sprintf("%s_BOOTSTRAP_IMAGE='%s'", envPrefix, param.Image),
		fmt.Sprintf("%s_BOOTSTRAP_POOL_ID='%s'", envPrefix, param.PoolID),
		fmt.Sprintf("%s_BOOTSTRAP_INSTANCE_TOKEN='%s'", envPrefix, param.InstanceToken),
		fmt.Sprintf("%s_BOOTSTRAP_CALLBACK_URL='%s'", envPrefix, param.CallbackURL),
		fmt.Sprintf("%s_BOOTSTRAP_REPO_URL='%s'", envPrefix, param.RepoURL),
		fmt.Sprintf("%s_BOOTSTRAP_LABELS='%s'", envPrefix, strings.Join(param.Labels, ",")),
		fmt.Sprintf("%s_BOOTSTRAP_GITHUB_ACCESS_TOKEN='%s'", envPrefix, param.GithubRunnerAccessToken),
	}

	for idx, tool := range param.Tools {
		ret = append(ret, fmt.Sprintf("%s_BOOTSTRAP_TOOLS_DOWNLOAD_URL_%d='%s'", envPrefix, idx, *tool.DownloadURL))
		ret = append(ret, fmt.Sprintf("%s_BOOTSTRAP_TOOLS_ARCH_%d='%s'", envPrefix, idx, *tool.Architecture))
		ret = append(ret, fmt.Sprintf("%s_BOOTSTRAP_TOOLS_OS_%d='%s'", envPrefix, idx, *tool.OS))
		ret = append(ret, fmt.Sprintf("%s_BOOTSTRAP_TOOLS_FILENAME_%d='%s'", envPrefix, idx, *tool.Filename))
		ret = append(ret, fmt.Sprintf("%s_BOOTSTRAP_TOOLS_SHA256_%d='%s'", envPrefix, idx, *tool.SHA256Checksum))
	}

	for idx, sshKey := range param.SSHKeys {
		ret = append(ret, fmt.Sprintf("%s_BOOTSTRAP_SSH_KEY_%d='%s'", envPrefix, idx, sshKey))

	}

	return ret
}
