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

package util

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode"

	"garm/cloudconfig"
	"garm/config"
	runnerErrors "garm/errors"
	"garm/params"
	"garm/runner/common"

	"github.com/google/go-github/v43/github"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const alphanumeric = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// From: https://www.alexedwards.net/blog/validation-snippets-for-go#email-validation
var rxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

var (
	OSToOSTypeMap map[string]config.OSType = map[string]config.OSType{
		"almalinux":  config.Linux,
		"alma":       config.Linux,
		"alpine":     config.Linux,
		"archlinux":  config.Linux,
		"arch":       config.Linux,
		"centos":     config.Linux,
		"ubuntu":     config.Linux,
		"rhel":       config.Linux,
		"suse":       config.Linux,
		"opensuse":   config.Linux,
		"fedora":     config.Linux,
		"debian":     config.Linux,
		"flatcar":    config.Linux,
		"gentoo":     config.Linux,
		"rockylinux": config.Linux,
		"rocky":      config.Linux,
		"windows":    config.Windows,
	}

	githubArchMapping map[string]string = map[string]string{
		"x86_64":  "x64",
		"amd64":   "x64",
		"armv7l":  "arm",
		"aarch64": "arm64",
		"x64":     "x64",
		"arm":     "arm",
		"arm64":   "arm64",
	}

	githubOSTypeMap map[string]string = map[string]string{
		"linux":   "linux",
		"windows": "win",
	}
)

func ResolveToGithubArch(arch string) (string, error) {
	ghArch, ok := githubArchMapping[arch]
	if !ok {
		return "", runnerErrors.NewNotFoundError("arch %s is unknown", arch)
	}

	return ghArch, nil
}

func ResolveToGithubOSType(osType string) (string, error) {
	ghOS, ok := githubOSTypeMap[osType]
	if !ok {
		return "", runnerErrors.NewNotFoundError("os %s is unknown", osType)
	}

	return ghOS, nil
}

// IsValidEmail returs a bool indicating if an email is valid
func IsValidEmail(email string) bool {
	if len(email) > 254 || !rxEmail.MatchString(email) {
		return false
	}
	return true
}

func IsAlphanumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			return false
		}
	}
	return true
}

// GetLoggingWriter returns a new io.Writer suitable for logging.
func GetLoggingWriter(cfg *config.Config) (io.Writer, error) {
	var writer io.Writer = os.Stdout
	if cfg.Default.LogFile != "" {
		dirname := path.Dir(cfg.Default.LogFile)
		if _, err := os.Stat(dirname); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to create log folder")
			}
			if err := os.MkdirAll(dirname, 0o711); err != nil {
				return nil, fmt.Errorf("failed to create log folder")
			}
		}
		writer = &lumberjack.Logger{
			Filename:   cfg.Default.LogFile,
			MaxSize:    500, // megabytes
			MaxBackups: 3,
			MaxAge:     28,   // days
			Compress:   true, // disabled by default
		}
	}
	return writer, nil
}

func ConvertFileToBase64(file string) (string, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return "", errors.Wrap(err, "reading file")
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}

func OSToOSType(os string) (config.OSType, error) {
	osType, ok := OSToOSTypeMap[strings.ToLower(os)]
	if !ok {
		return config.Unknown, fmt.Errorf("no OS to OS type mapping for %s", os)
	}
	return osType, nil
}

func GithubClient(ctx context.Context, token string) (common.GithubClient, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	tc := oauth2.NewClient(ctx, ts)

	var ghClient *github.Client
	var err error
	if config.IsGithubEnterprise {
		ghClient, err = github.NewEnterpriseClient(config.GithubBaseURL, config.GithubBaseURL, tc)
		log.Printf("Using GitHub Enterprise at %s", config.GithubBaseURL)
		if err != nil {
			return nil, err
		}
	} else {
		ghClient = github.NewClient(tc)
	}

	return ghClient.Actions, nil
}

func GetCloudConfig(bootstrapParams params.BootstrapInstance, tools github.RunnerApplicationDownload, runnerName string) (string, error) {
	cloudCfg := cloudconfig.NewDefaultCloudInitConfig()

	installRunnerParams := cloudconfig.InstallRunnerParams{
		FileName:       *tools.Filename,
		DownloadURL:    *tools.DownloadURL,
		GithubToken:    bootstrapParams.GithubRunnerAccessToken,
		RunnerUsername: config.DefaultUser,
		RunnerGroup:    config.DefaultUser,
		RepoURL:        bootstrapParams.RepoURL,
		RunnerName:     runnerName,
		RunnerLabels:   strings.Join(bootstrapParams.Labels, ","),
		CallbackURL:    bootstrapParams.CallbackURL,
		CallbackToken:  bootstrapParams.InstanceToken,
	}

	installScript, err := cloudconfig.InstallRunnerScript(installRunnerParams)
	if err != nil {
		return "", errors.Wrap(err, "generating script")
	}

	cloudCfg.AddSSHKey(bootstrapParams.SSHKeys...)
	cloudCfg.AddFile(installScript, "/install_runner.sh", "root:root", "755")
	cloudCfg.AddRunCmd("/install_runner.sh")
	cloudCfg.AddRunCmd("rm -f /install_runner.sh")

	asStr, err := cloudCfg.Serialize()
	if err != nil {
		return "", errors.Wrap(err, "creating cloud config")
	}
	return asStr, nil
}

// GetRandomString returns a secure random string
func GetRandomString(n int) (string, error) {
	data := make([]byte, n)
	_, err := rand.Read(data)
	if err != nil {
		return "", errors.Wrap(err, "getting random data")
	}
	for i, b := range data {
		data[i] = alphanumeric[b%byte(len(alphanumeric))]
	}

	return string(data), nil
}

func Aes256EncodeString(target string, passphrase string) ([]byte, error) {
	if len(passphrase) != 32 {
		return nil, fmt.Errorf("invalid passphrase length (expected length 32 characters)")
	}

	toEncrypt := []byte(target)
	block, err := aes.NewCipher([]byte(passphrase))
	if err != nil {
		return nil, errors.Wrap(err, "creating cipher")
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating new aead")
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.Wrap(err, "creating nonce")
	}

	ciphertext := aesgcm.Seal(nonce, nonce, toEncrypt, nil)
	return ciphertext, nil
}

func Aes256DecodeString(target []byte, passphrase string) (string, error) {
	if len(passphrase) != 32 {
		return "", fmt.Errorf("invalid passphrase length (expected length 32 characters)")
	}

	block, err := aes.NewCipher([]byte(passphrase))
	if err != nil {
		return "", errors.Wrap(err, "creating cipher")
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", errors.Wrap(err, "creating new aead")
	}

	nonceSize := aesgcm.NonceSize()
	if len(target) < nonceSize {
		return "", fmt.Errorf("failed to decrypt text")
	}

	nonce, ciphertext := target[:nonceSize], target[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt text")
	}
	return string(plaintext), nil
}

// PaswsordToBcrypt returns a bcrypt hash of the specified password using the default cost
func PaswsordToBcrypt(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password")
	}
	return string(hashedPassword), nil
}
