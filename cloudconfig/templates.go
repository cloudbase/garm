package cloudconfig

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"
)

var CloudConfigTemplate = `
#!/bin/bash

set -ex

curl -o "/home/runner/{{ .FileName }}" "{{ .DownloadURL }}"
mkdir -p /home/runner/action-runner
tar xf "/home/runner/{{ .FileName }}" -C /home/runner/action-runner/
chown {{ .RunnerUsername }}:{{ .RunnerGroup }} -R /home/{{ .RunnerUsername }}/action-runner/
sudo /home/{{ .RunnerUsername }}/actions-runner/bin/installdependencies.sh
sudo -u {{ .RunnerUsername }} -- /home/{{ .RunnerUsername }}/actions-runner/config.sh --unattended --url "{{ .RepoURL }}" --token "{{ .GithubToken }}" --name "{{ .RunnerName }}" --labels "{{ .RunnerLabels }}" --ephemeral
/home/{{ .RunnerUsername }}/actions-runner/svc.sh install
/home/{{ .RunnerUsername }}/actions-runner/svc.sh start
`

type InstallRunnerParams struct {
	FileName       string
	DownloadURL    string
	RunnerUsername string
	RunnerGroup    string
	RepoURL        string
	GithubToken    string
	RunnerName     string
	RunnerLabels   string
}

func InstallRunnerScript(params InstallRunnerParams) ([]byte, error) {

	t, err := template.New("").Parse(CloudConfigTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "parsing template")
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, params); err != nil {
		return nil, errors.Wrap(err, "rendering template")
	}

	return buf.Bytes(), nil
}
