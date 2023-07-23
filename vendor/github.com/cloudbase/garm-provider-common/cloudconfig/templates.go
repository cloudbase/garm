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
	"fmt"
	"text/template"

	"github.com/cloudbase/garm-provider-common/params"
	"github.com/pkg/errors"
)

var CloudConfigTemplate = `#!/bin/bash

set -e
set -o pipefail

{{- if .EnableBootDebug }}
set -x
{{- end }}

CALLBACK_URL="{{ .CallbackURL }}"
METADATA_URL="{{ .MetadataURL }}"
BEARER_TOKEN="{{ .CallbackToken }}"

if [ -z "$METADATA_URL" ];then
	echo "no token is available and METADATA_URL is not set"
	exit 1
fi
GITHUB_TOKEN=$(curl --retry 5 --retry-delay 5 --retry-connrefused --fail -s -X GET -H 'Accept: application/json' -H "Authorization: Bearer ${BEARER_TOKEN}" "${METADATA_URL}/runner-registration-token/")

function call() {
	PAYLOAD="$1"
	[[ $CALLBACK_URL =~ ^(.*)/status$ ]]
	if [ -z "$BASH_REMATCH" ];then
		CALLBACK_URL="${CALLBACK_URL}/status"
	fi
	curl --retry 5 --retry-delay 5 --retry-connrefused --fail -s -X POST -d "${PAYLOAD}" -H 'Accept: application/json' -H "Authorization: Bearer ${BEARER_TOKEN}" "${CALLBACK_URL}" || echo "failed to call home: exit code ($?)"
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

# This will echo the version number in the filename. Given a file name like: actions-runner-osx-x64-2.299.1.tar.gz
# this will output: 2.299.1
function getRunnerVersion() {
	FILENAME="{{ .FileName }}"
	[[ $FILENAME =~ ([0-9]+\.[0-9]+\.[0-9+]) ]]
	echo $BASH_REMATCH
}

function getCachedToolsPath() {
	CACHED_RUNNER="/opt/cache/actions-runner/latest"
	if [ -d "$CACHED_RUNNER" ];then
		echo "$CACHED_RUNNER"
		return 0
	fi

	VERSION=$(getRunnerVersion)
	if [ -z "$VERSION" ]; then
		return 0
	fi

	CACHED_RUNNER="/opt/cache/actions-runner/$VERSION"
	if [ -d "$CACHED_RUNNER" ];then
		echo "$CACHED_RUNNER"
		return 0
	fi
	return 0
}

function downloadAndExtractRunner() {
	sendStatus "downloading tools from {{ .DownloadURL }}"
	if [ ! -z "{{ .TempDownloadToken }}" ]; then
	TEMP_TOKEN="Authorization: Bearer {{ .TempDownloadToken }}"
	fi
	curl --retry 5 --retry-delay 5 --retry-connrefused --fail -L -H "${TEMP_TOKEN}" -o "/home/{{ .RunnerUsername }}/{{ .FileName }}" "{{ .DownloadURL }}" || fail "failed to download tools"
	mkdir -p /home/{{ .RunnerUsername }}/actions-runner || fail "failed to create actions-runner folder"
	sendStatus "extracting runner"
	tar xf "/home/{{ .RunnerUsername }}/{{ .FileName }}" -C /home/{{ .RunnerUsername }}/actions-runner/ || fail "failed to extract runner"
	# chown {{ .RunnerUsername }}:{{ .RunnerGroup }} -R /home/{{ .RunnerUsername }}/actions-runner/ || fail "failed to change owner"
}

TEMP_TOKEN=""
GH_RUNNER_GROUP="{{.GitHubRunnerGroup}}"

# $RUNNER_GROUP_OPT will be added to the config.sh line. If it's empty, nothing happens
# if it holds a value, it will be part of the command.
RUNNER_GROUP_OPT=""
if [ ! -z $GH_RUNNER_GROUP ];then
	RUNNER_GROUP_OPT="--runnergroup=$GH_RUNNER_GROUP"
fi

CACHED_RUNNER=$(getCachedToolsPath)
if [ -z "$CACHED_RUNNER" ];then
	downloadAndExtractRunner
	sendStatus "installing dependencies"
	cd /home/{{ .RunnerUsername }}/actions-runner
	sudo ./bin/installdependencies.sh || fail "failed to install dependencies"
else
	sendStatus "using cached runner found in $CACHED_RUNNER"
	sudo cp -a "$CACHED_RUNNER"  "/home/{{ .RunnerUsername }}/actions-runner"
	sudo chown {{ .RunnerUsername }}:{{ .RunnerGroup }} -R "/home/{{ .RunnerUsername }}/actions-runner" || fail "failed to change owner"
	cd /home/{{ .RunnerUsername }}/actions-runner
fi


sendStatus "configuring runner"
set +e
attempt=1
while true; do
	ERROUT=$(mktemp)
	./config.sh --unattended --url "{{ .RepoURL }}" --token "$GITHUB_TOKEN" $RUNNER_GROUP_OPT --name "{{ .RunnerName }}" --labels "{{ .RunnerLabels }}" --ephemeral 2>$ERROUT
	if [ $? -eq 0 ]; then
		rm $ERROUT || true
		sendStatus "runner successfully configured after $attempt attempt(s)"
		break
	fi
	LAST_ERR=$(cat $ERROUT)
	echo "$LAST_ERR"

	# if the runner is already configured, remove it and try again. In the past configuring a runner
	# managed to register it but timed out later, resulting in an error.
	./config.sh remove --token "$GITHUB_TOKEN" || true

	if [ $attempt -gt 5 ];then
		rm $ERROUT || true
		fail "failed to configure runner: $LAST_ERR"
	fi

	sendStatus "failed to configure runner (attempt $attempt): $LAST_ERR (retrying in 5 seconds)"
	attempt=$((attempt+1))
	rm $ERROUT || true
	sleep 5
done
set -e

sendStatus "installing runner service"
sudo ./svc.sh install {{ .RunnerUsername }} || fail "failed to install service"

if [ -e "/sys/fs/selinux" ];then
	sudo chcon -h user_u:object_r:bin_t /home/runner/ || fail "failed to change selinux context"
	sudo chcon -R -h {{ .RunnerUsername }}:object_r:bin_t /home/runner/* || fail "failed to change selinux context"
fi

sendStatus "starting service"
sudo ./svc.sh start || fail "failed to start service"

set +e
AGENT_ID=$(grep "agentId" /home/{{ .RunnerUsername }}/actions-runner/.runner |  tr -d -c 0-9)
if [ $? -ne 0 ];then
	fail "failed to get agent ID"
fi
set -e

success "runner successfully installed" $AGENT_ID
`

var WindowsSetupScriptTemplate = `#ps1_sysnative
Param(
	[Parameter(Mandatory=$false)]
	[string]$Token="{{.CallbackToken}}"
)

$ErrorActionPreference="Stop"

function Invoke-FastWebRequest {
	[CmdletBinding()]
	Param(
		[Parameter(Mandatory=$True,ValueFromPipeline=$true,Position=0)]
		[System.Uri]$Uri,
		[Parameter(Position=1)]
		[string]$OutFile,
		[Hashtable]$Headers=@{},
		[switch]$SkipIntegrityCheck=$false
	)
	PROCESS
	{
		if(!([System.Management.Automation.PSTypeName]'System.Net.Http.HttpClient').Type)
		{
			$assembly = [System.Reflection.Assembly]::LoadWithPartialName("System.Net.Http")
		}

		if(!$OutFile) {
			$OutFile = $Uri.PathAndQuery.Substring($Uri.PathAndQuery.LastIndexOf("/") + 1)
			if(!$OutFile) {
				throw "The ""OutFile"" parameter needs to be specified"
			}
		}

		$fragment = $Uri.Fragment.Trim('#')
		if ($fragment) {
			$details = $fragment.Split("=")
			$algorithm = $details[0]
			$hash = $details[1]
		}

		if (!$SkipIntegrityCheck -and $fragment -and (Test-Path $OutFile)) {
			try {
				return (Test-FileIntegrity -File $OutFile -Algorithm $algorithm -ExpectedHash $hash)
			} catch {
				Remove-Item $OutFile
			}
		}

		$client = new-object System.Net.Http.HttpClient
		foreach ($k in $Headers.Keys){
			$client.DefaultRequestHeaders.Add($k, $Headers[$k])
		}
		$task = $client.GetStreamAsync($Uri)
		$response = $task.Result
		if($task.IsFaulted) {
			$msg = "Request for URL '{0}' is faulted. Task status: {1}." -f @($Uri, $task.Status)
			if($task.Exception) {
				$msg += "Exception details: {0}" -f @($task.Exception)
			}
			Throw $msg
		}
		$outStream = New-Object IO.FileStream $OutFile, Create, Write, None

		try {
			$totRead = 0
			$buffer = New-Object Byte[] 1MB
			while (($read = $response.Read($buffer, 0, $buffer.Length)) -gt 0) {
				$totRead += $read
				$outStream.Write($buffer, 0, $read);
			}
		}
		finally {
			$outStream.Close()
		}
		if(!$SkipIntegrityCheck -and $fragment) {
			Test-FileIntegrity -File $OutFile -Algorithm $algorithm -ExpectedHash $hash
		}
	}
}

function Import-Certificate() {
	[CmdletBinding()]
	param (
		[parameter(Mandatory=$true)]
		[string]$CertificatePath,
		[parameter(Mandatory=$true)]
		[System.Security.Cryptography.X509Certificates.StoreLocation]$StoreLocation="LocalMachine",
		[parameter(Mandatory=$true)]
		[System.Security.Cryptography.X509Certificates.StoreName]$StoreName="TrustedPublisher"
	)
	PROCESS
	{
		$store = New-Object System.Security.Cryptography.X509Certificates.X509Store(
			$StoreName, $StoreLocation)
		$store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadWrite)
		$cert = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2(
			$CertificatePath)
		$store.Add($cert)
	}
}

function Invoke-APICall() {
	[CmdletBinding()]
	param (
		[parameter(Mandatory=$true)]
		[object]$Payload,
		[parameter(Mandatory=$true)]
		[string]$CallbackURL
	)
	PROCESS{
		Invoke-WebRequest -UseBasicParsing -Method Post -Headers @{"Accept"="application/json"; "Authorization"="Bearer $Token"} -Uri $CallbackURL -Body (ConvertTo-Json $Payload) | Out-Null
	}
}

function Update-GarmStatus() {
	[CmdletBinding()]
	param (
		[parameter(Mandatory=$true)]
		[string]$Message,
		[parameter(Mandatory=$true)]
		[string]$CallbackURL
	)
	PROCESS{
		$body = @{
			"status"="installing"
			"message"=$Message
		}
		Invoke-APICall -Payload $body -CallbackURL $CallbackURL | Out-Null
	}
}

function Invoke-GarmSuccess() {
	[CmdletBinding()]
	param (
		[parameter(Mandatory=$true)]
		[string]$Message,
		[parameter(Mandatory=$true)]
		[int64]$AgentID,
		[parameter(Mandatory=$true)]
		[string]$CallbackURL
	)
	PROCESS{
		$body = @{
			"status"="idle"
			"message"=$Message
			"agent_id"=$AgentID
		}
		Invoke-APICall -Payload $body -CallbackURL $CallbackURL | Out-Null
	}
}

function Invoke-GarmFailure() {
	[CmdletBinding()]
	param (
		[parameter(Mandatory=$true)]
		[string]$Message,
		[parameter(Mandatory=$true)]
		[string]$CallbackURL
	)
	PROCESS{
		$body = @{
			"status"="failed"
			"message"=$Message
		}
		Invoke-APICall -Payload $body -CallbackURL $CallbackURL | Out-Null
		Throw $Message
	}
}

$PEMData = @"
{{.CABundle}}
"@
$GHRunnerGroup = "{{.GitHubRunnerGroup}}"

function Install-Runner() {
	$CallbackURL="{{.CallbackURL}}"
	if (!$CallbackURL.EndsWith("/status")) {
		$CallbackURL = "$CallbackURL/status"
	}

	if ($Token.Length -eq 0) {
		Throw "missing callback authentication token"
	}
	try {
		$MetadataURL="{{.MetadataURL}}"
		$DownloadURL="{{.DownloadURL}}"
		if($MetadataURL -eq ""){
			Throw "missing metadata URL"
		}

		if($PEMData.Trim().Length -gt 0){
			Set-Content $env:TMP\garm-ca.pem $PEMData
			Import-Certificate -CertificatePath $env:TMP\garm-ca.pem
		}

		$GithubRegistrationToken = Invoke-WebRequest -UseBasicParsing -Headers @{"Accept"="application/json"; "Authorization"="Bearer $Token"} -Uri $MetadataURL/runner-registration-token/
		Update-GarmStatus -CallbackURL $CallbackURL -Message "downloading tools from $DownloadURL"

		$downloadToken="{{.TempDownloadToken}}"
		$DownloadTokenHeaders=@{}
		if ($downloadToken.Length -gt 0) {
			$DownloadTokenHeaders=@{
				"Authorization"="Bearer $downloadToken"
			}
		}
		$downloadPath = Join-Path $env:TMP {{.FileName}}
		Invoke-FastWebRequest -Uri $DownloadURL -OutFile $downloadPath -Headers $DownloadTokenHeaders

		$runnerDir = "C:\runner"
		mkdir $runnerDir

		Update-GarmStatus -CallbackURL $CallbackURL -Message "extracting runner"
		Add-Type -AssemblyName System.IO.Compression.FileSystem
		[System.IO.Compression.ZipFile]::ExtractToDirectory($downloadPath, "$runnerDir")
		$runnerGroupOpt = ""
		if ($GHRunnerGroup.Length -gt 0){
			$runnerGroupOpt = "--runnergroup $GHRunnerGroup"
		}
		Update-GarmStatus -CallbackURL $CallbackURL -Message "configuring and starting runner"
		cd $runnerDir
		./config.cmd --unattended --url "{{ .RepoURL }}" --token $GithubRegistrationToken $runnerGroupOpt --name "{{ .RunnerName }}" --labels "{{ .RunnerLabels }}" --ephemeral --runasservice

		$agentInfoFile = Join-Path $runnerDir ".runner"
		$agentInfo = ConvertFrom-Json (gc -raw $agentInfoFile)
		Invoke-GarmSuccess -CallbackURL $CallbackURL -Message "runner successfully installed" -AgentID $agentInfo.agentId
	} catch {
		Invoke-GarmFailure -CallbackURL $CallbackURL -Message $_
	}
}
Install-Runner
`

// InstallRunnerParams holds the parameters needed to render the runner install script.
type InstallRunnerParams struct {
	// FileName is the name of the file that will be downloaded from the download URL.
	// This will be the runner archive downloaded from GitHub.
	FileName string
	// DownloadURL is the URL from which the runner archive will be downloaded.
	DownloadURL string
	// RunnerUsername is the username of the user that will run the runner service.
	RunnerUsername string
	// RunnerGroup is the group of the user that will run the runner service.
	RunnerGroup string
	// RepoURL is the URL or the github repo the github runner agent needs to configure itself.
	RepoURL string
	// MetadataURL is the URL where instances can fetch information needed to set themselves up.
	// This URL is set in the GARM config file.
	MetadataURL string
	// RunnerName is the name of the runner. GARM will use this to register the runner with GitHub.
	RunnerName string
	// RunnerLabels is a comma separated list of labels that will be added to the runner.
	RunnerLabels string
	// CallbackURL is the URL where the instance can send a post, signaling progress or status.
	// This URL is set in the GARM config file.
	CallbackURL string
	// CallbackToken is the token that needs to be set by the instance in the headers in order to call
	// the CallbackURL.
	CallbackToken string
	// TempDownloadToken is the token that needs to be set by the instance in the headers in order to download
	// the githun runner. This is usually needed when using garm against a GHES instance.
	TempDownloadToken string
	// CABundle is a CA certificate bundle which will be sent to instances and which will tipically be installed
	// as a system wide trusted root CA by either cloud-init or whatever mechanism the provider will use to set
	// up the runner.
	CABundle string
	// GitHubRunnerGroup is the github runner group in which the newly installed runner should be added to.
	GitHubRunnerGroup string
	// EnableBootDebug will enable bash debug mode.
	EnableBootDebug bool
	// ExtraContext is a map of extra context that will be passed to the runner install template.
	// This option is useful for situations in which you're supplying your own template and you need
	// to pass in information that is not available in the default template.
	ExtraContext map[string]string
}

func InstallRunnerScript(installParams InstallRunnerParams, osType params.OSType, tpl string) ([]byte, error) {
	if tpl == "" {
		switch osType {
		case params.Linux:
			tpl = CloudConfigTemplate
		case params.Windows:
			tpl = WindowsSetupScriptTemplate
		default:
			return nil, fmt.Errorf("unsupported os type: %s", osType)
		}
	}

	t, err := template.New("").Parse(tpl)
	if err != nil {
		return nil, errors.Wrap(err, "parsing template")
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, installParams); err != nil {
		return nil, errors.Wrap(err, "rendering template")
	}

	return buf.Bytes(), nil
}
