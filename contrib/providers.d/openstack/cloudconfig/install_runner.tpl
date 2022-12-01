#!/bin/bash

set -ex
set -o pipefail

CALLBACK_URL="GARM_CALLBACK_URL"
BEARER_TOKEN="GARM_CALLBACK_TOKEN"
DOWNLOAD_URL="GH_DOWNLOAD_URL"
DOWNLOAD_TOKEN="GH_TEMP_DOWNLOAD_TOKEN"
FILENAME="GH_FILENAME"
TARGET_URL="GH_TARGET_URL"
RUNNER_TOKEN="GH_RUNNER_TOKEN"
RUNNER_NAME="GH_RUNNER_NAME"
RUNNER_LABELS="GH_RUNNER_LABELS"
TEMP_TOKEN=""

if [ ! -z "$DOWNLOAD_TOKEN" ]; then
	TEMP_TOKEN="Authorization: Bearer $DOWNLOAD_TOKEN"
fi


function call() {
	PAYLOAD="$1"
	curl -s -X POST -d "${PAYLOAD}" -H 'Accept: application/json' -H "Authorization: Bearer ${BEARER_TOKEN}" "${CALLBACK_URL}" || echo "failed to call home: exit code ($?)"
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



sendStatus "downloading tools from ${DOWNLOAD_URL}"
curl -L -H "${TEMP_TOKEN}" -o "/home/runner/${FILENAME}" "${DOWNLOAD_URL}" || fail "failed to download tools"

mkdir -p /home/runner/actions-runner || fail "failed to create actions-runner folder"

sendStatus "extracting runner"
tar xf "/home/runner/${FILENAME}" -C /home/runner/actions-runner/ || fail "failed to extract runner"
chown runner:runner -R /home/runner/actions-runner/ || fail "failed to change owner"

sendStatus "installing dependencies"
cd /home/runner/actions-runner
sudo ./bin/installdependencies.sh || fail "failed to install dependencies"

sendStatus "configuring runner"
sudo -u runner -- ./config.sh --unattended --url "${TARGET_URL}" --token "${RUNNER_TOKEN}" --name "${RUNNER_NAME}" --labels "${RUNNER_LABELS}" --ephemeral || fail "failed to configure runner"

sendStatus "installing runner service"
./svc.sh install runner || fail "failed to install service"

sendStatus "starting service"
./svc.sh start || fail "failed to start service"

set +e
AGENT_ID=$(grep "agentId" /home/runner/actions-runner/.runner |  tr -d -c 0-9)
if [ $? -ne 0 ];then
	fail "failed to get agent ID"
fi
set -e

success "runner successfully installed" $AGENT_ID
