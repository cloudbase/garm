package cloudconfig

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"
)

var CloudConfigTemplate = `#!/bin/bash

set -ex
set -o pipefail

CALLBACK_URL="{{ .CallbackURL }}"
BEARER_TOKEN="{{ .CallbackToken }}"

function call() {
	PAYLOAD="$1"
	curl -s -X POST -d \'${PAYLOAD}\' -H 'Accept: application/json' -H "Authorization: Bearer ${BEARER_TOKEN}" "${CALLBACK_URL}" || echo "failed to call home: exit code ($?)"
}

function sendStatus() {
	MSG="$1"
	call '{"status": "installing", "message": "'$MSG'"}'
}

function success() {
	MSG="$1"
	call '{"status": "active", "message": "'$MSG'"}'
}

function fail() {
	MSG="$1"
	call '{"status": "failed", "message": "'$MSG'"}'
	exit 1
}

sendStatus "downloading tools from {{ .DownloadURL }}"
curl -L -o "/home/runner/{{ .FileName }}" "{{ .DownloadURL }}" || fail "failed to download tools"

mkdir -p /home/runner/actions-runner || fail "failed to create actions-runner folder"

sendStatus "extracting runner"
tar xf "/home/runner/{{ .FileName }}" -C /home/runner/actions-runner/ || fail "failed to extract runner"
chown {{ .RunnerUsername }}:{{ .RunnerGroup }} -R /home/{{ .RunnerUsername }}/actions-runner/ || fail "failed to change owner"

sendStatus "installing dependencies"
cd /home/{{ .RunnerUsername }}/actions-runner
sudo ./bin/installdependencies.sh || fail "failed to install dependencies"

sendStatus "configuring runner"
sudo -u {{ .RunnerUsername }} -- ./config.sh --unattended --url "{{ .RepoURL }}" --token "{{ .GithubToken }}" --name "{{ .RunnerName }}" --labels "{{ .RunnerLabels }}" --ephemeral || fail "failed to configure runner"

sendStatus "installing runner service"
./svc.sh install {{ .RunnerUsername }} || fail "failed to install service"

sendStatus "starting service"
./svc.sh start || fail "failed to start service"

success "runner successfully installed"
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
	CallbackURL    string
	CallbackToken  string
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
