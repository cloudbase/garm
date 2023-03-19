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

package cloudconfig

import (
	"bytes"
	"text/template"

	"github.com/pkg/errors"
)

var CloudConfigTemplate = `#!/bin/bash

set -e
set -o pipefail

CALLBACK_URL="{{ .CallbackURL }}"
METADATA_URL="{{ .MetadataURL }}"
BEARER_TOKEN="{{ .CallbackToken }}"

if [ -z "$METADATA_URL" ];then
	echo "no token is available and METADATA_URL is not set"
	exit 1
fi
GITHUB_TOKEN=$(curl --fail -s -X GET -H 'Accept: application/json' -H "Authorization: Bearer ${BEARER_TOKEN}" "${METADATA_URL}/runner-registration-token/")

function call() {
	PAYLOAD="$1"
	curl --fail -s -X POST -d "${PAYLOAD}" -H 'Accept: application/json' -H "Authorization: Bearer ${BEARER_TOKEN}" "${CALLBACK_URL}" || echo "failed to call home: exit code ($?)"
}

function sendStatus() {
	MSG="$1"
	call "{\"status\": \"installing\", \"message\": \"$MSG\"}"
}

function success() {
	MSG="$1"
	ID=$2
	call "{\"status\": \"idle\", \"message\": \"$MSG\", \"agent_id\": $ID}"
}

function fail() {
	MSG="$1"
	call "{\"status\": \"failed\", \"message\": \"$MSG\"}"
	exit 1
}

sendStatus "downloading tools from {{ .DownloadURL }}"

TEMP_TOKEN=""

if [ ! -z "{{ .TempDownloadToken }}" ]; then
	TEMP_TOKEN="Authorization: Bearer {{ .TempDownloadToken }}"
fi

curl -L -H "${TEMP_TOKEN}" -o "/home/{{ .RunnerUsername }}/{{ .FileName }}" "{{ .DownloadURL }}" || fail "failed to download tools"

mkdir -p /home/runner/actions-runner || fail "failed to create actions-runner folder"

sendStatus "extracting runner"
tar xf "/home/{{ .RunnerUsername }}/{{ .FileName }}" -C /home/{{ .RunnerUsername }}/actions-runner/ || fail "failed to extract runner"
chown {{ .RunnerUsername }}:{{ .RunnerGroup }} -R /home/{{ .RunnerUsername }}/actions-runner/ || fail "failed to change owner"

sendStatus "installing dependencies"
cd /home/{{ .RunnerUsername }}/actions-runner
sudo ./bin/installdependencies.sh || fail "failed to install dependencies"

sendStatus "configuring runner"
sudo -u {{ .RunnerUsername }} -- ./config.sh --unattended --url "{{ .RepoURL }}" --token "$GITHUB_TOKEN" --name "{{ .RunnerName }}" --labels "{{ .RunnerLabels }}" --ephemeral || fail "failed to configure runner"

sendStatus "installing runner service"
./svc.sh install {{ .RunnerUsername }} || fail "failed to install service"

if [ -e "/sys/fs/selinux" ];then
    sudo chcon -R -t bin_t /home/runner/
fi

sendStatus "starting service"
./svc.sh start || fail "failed to start service"

set +e
AGENT_ID=$(grep "agentId" /home/{{ .RunnerUsername }}/actions-runner/.runner |  tr -d -c 0-9)
if [ $? -ne 0 ];then
	fail "failed to get agent ID"
fi
set -e

success "runner successfully installed" $AGENT_ID
`

type InstallRunnerParams struct {
	FileName          string
	DownloadURL       string
	RunnerUsername    string
	RunnerGroup       string
	RepoURL           string
	MetadataURL       string
	RunnerName        string
	RunnerLabels      string
	CallbackURL       string
	CallbackToken     string
	TempDownloadToken string
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
