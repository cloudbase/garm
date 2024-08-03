// Copyright 2023 Cloudbase Solutions SRL
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

package params

// RunnerApplicationDownload represents a binary for the self-hosted runner application that can be downloaded.
// This is copied from the go-github package. It does not make sense to create a dependency on go-github just
// for this struct.
type RunnerApplicationDownload struct {
	OS                *string `json:"os,omitempty"`
	Architecture      *string `json:"architecture,omitempty"`
	DownloadURL       *string `json:"download_url,omitempty"`
	Filename          *string `json:"filename,omitempty"`
	TempDownloadToken *string `json:"temp_download_token,omitempty"`
	SHA256Checksum    *string `json:"sha256_checksum,omitempty"`
}

// GetArchitecture returns the Architecture field if it's non-nil, zero value otherwise.
func (r *RunnerApplicationDownload) GetArchitecture() string {
	if r == nil || r.Architecture == nil {
		return ""
	}
	return *r.Architecture
}

// GetDownloadURL returns the DownloadURL field if it's non-nil, zero value otherwise.
func (r *RunnerApplicationDownload) GetDownloadURL() string {
	if r == nil || r.DownloadURL == nil {
		return ""
	}
	return *r.DownloadURL
}

// GetFilename returns the Filename field if it's non-nil, zero value otherwise.
func (r *RunnerApplicationDownload) GetFilename() string {
	if r == nil || r.Filename == nil {
		return ""
	}
	return *r.Filename
}

// GetOS returns the OS field if it's non-nil, zero value otherwise.
func (r *RunnerApplicationDownload) GetOS() string {
	if r == nil || r.OS == nil {
		return ""
	}
	return *r.OS
}

// GetSHA256Checksum returns the SHA256Checksum field if it's non-nil, zero value otherwise.
func (r *RunnerApplicationDownload) GetSHA256Checksum() string {
	if r == nil || r.SHA256Checksum == nil {
		return ""
	}
	return *r.SHA256Checksum
}

// GetTempDownloadToken returns the TempDownloadToken field if it's non-nil, zero value otherwise.
func (r *RunnerApplicationDownload) GetTempDownloadToken() string {
	if r == nil || r.TempDownloadToken == nil {
		return ""
	}
	return *r.TempDownloadToken
}
