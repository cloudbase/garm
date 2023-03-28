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
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/cloudbase/garm/cloudconfig"
	"github.com/cloudbase/garm/config"
	runnerErrors "github.com/cloudbase/garm/errors"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
	"github.com/cloudbase/garm/util/appdefaults"

	"github.com/google/go-github/v48/github"
	"github.com/google/uuid"
	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/pkg/errors"
	"github.com/teris-io/shortid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const alphanumeric = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// From: https://www.alexedwards.net/blog/validation-snippets-for-go#email-validation
var rxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

var (
	OSToOSTypeMap map[string]params.OSType = map[string]params.OSType{
		"almalinux":  params.Linux,
		"alma":       params.Linux,
		"alpine":     params.Linux,
		"archlinux":  params.Linux,
		"arch":       params.Linux,
		"centos":     params.Linux,
		"ubuntu":     params.Linux,
		"rhel":       params.Linux,
		"suse":       params.Linux,
		"opensuse":   params.Linux,
		"fedora":     params.Linux,
		"debian":     params.Linux,
		"flatcar":    params.Linux,
		"gentoo":     params.Linux,
		"rockylinux": params.Linux,
		"rocky":      params.Linux,
		"windows":    params.Windows,
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

	//
	githubOSTag = map[params.OSType]string{
		params.Linux:   "Linux",
		params.Windows: "Windows",
	}
)

// ResolveToGithubArch returns the cpu architecture as it is defined in the GitHub
// tools download list. We use it to find the proper tools for the OS/Arch combo we're
// deploying.
func ResolveToGithubArch(arch string) (string, error) {
	ghArch, ok := githubArchMapping[arch]
	if !ok {
		return "", runnerErrors.NewNotFoundError("arch %s is unknown", arch)
	}

	return ghArch, nil
}

// ResolveToGithubArch returns the OS type as it is defined in the GitHub
// tools download list. We use it to find the proper tools for the OS/Arch combo we're
// deploying.
func ResolveToGithubOSType(osType string) (string, error) {
	ghOS, ok := githubOSTypeMap[osType]
	if !ok {
		return "", runnerErrors.NewNotFoundError("os %s is unknown", osType)
	}

	return ghOS, nil
}

// ResolveToGithubTag returns the default OS tag that self hosted runners automatically
// (and forcefully) adds to every runner that gets deployed. We need to keep track of those
// tags internally as well.
func ResolveToGithubTag(os params.OSType) (string, error) {
	ghOS, ok := githubOSTag[os]
	if !ok {
		return "", runnerErrors.NewNotFoundError("os %s is unknown", os)
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
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		}
	}
	return writer, nil
}

func ConvertFileToBase64(file string) (string, error) {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return "", errors.Wrap(err, "reading file")
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}

func OSToOSType(os string) (params.OSType, error) {
	osType, ok := OSToOSTypeMap[strings.ToLower(os)]
	if !ok {
		return params.Unknown, fmt.Errorf("no OS to OS type mapping for %s", os)
	}
	return osType, nil
}

func GithubClient(ctx context.Context, token string, credsDetails params.GithubCredentials) (common.GithubClient, common.GithubEnterpriseClient, error) {
	var roots *x509.CertPool
	if credsDetails.CABundle != nil && len(credsDetails.CABundle) > 0 {
		roots = x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(credsDetails.CABundle)
		if !ok {
			return nil, nil, fmt.Errorf("failed to parse CA cert")
		}
	}
	httpTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			ClientCAs: roots,
		},
	}
	httpClient := &http.Client{Transport: httpTransport}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	ghClient, err := github.NewEnterpriseClient(credsDetails.APIBaseURL, credsDetails.UploadBaseURL, tc)
	if err != nil {
		return nil, nil, errors.Wrap(err, "fetching github client")
	}

	return ghClient.Actions, ghClient.Enterprise, nil
}

func GetCloudConfig(bootstrapParams params.BootstrapInstance, tools github.RunnerApplicationDownload, runnerName string) (string, error) {
	if tools.Filename == nil {
		return "", fmt.Errorf("missing tools filename")
	}

	if tools.DownloadURL == nil {
		return "", fmt.Errorf("missing tools download URL")
	}

	var tempToken string
	if tools.TempDownloadToken != nil {
		tempToken = *tools.TempDownloadToken
	}

	installRunnerParams := cloudconfig.InstallRunnerParams{
		FileName:          *tools.Filename,
		DownloadURL:       *tools.DownloadURL,
		TempDownloadToken: tempToken,
		MetadataURL:       bootstrapParams.MetadataURL,
		RunnerUsername:    appdefaults.DefaultUser,
		RunnerGroup:       appdefaults.DefaultUser,
		RepoURL:           bootstrapParams.RepoURL,
		RunnerName:        runnerName,
		RunnerLabels:      strings.Join(bootstrapParams.Labels, ","),
		CallbackURL:       bootstrapParams.CallbackURL,
		CallbackToken:     bootstrapParams.InstanceToken,
		GitHubRunnerGroup: bootstrapParams.GitHubRunnerGroup,
	}
	if bootstrapParams.CACertBundle != nil && len(bootstrapParams.CACertBundle) > 0 {
		installRunnerParams.CABundle = string(bootstrapParams.CACertBundle)
	}

	installScript, err := cloudconfig.InstallRunnerScript(installRunnerParams, bootstrapParams.OSType)
	if err != nil {
		return "", errors.Wrap(err, "generating script")
	}

	var asStr string
	switch bootstrapParams.OSType {
	case params.Linux:
		cloudCfg := cloudconfig.NewDefaultCloudInitConfig()
		cloudCfg.AddSSHKey(bootstrapParams.SSHKeys...)
		cloudCfg.AddFile(installScript, "/install_runner.sh", "root:root", "755")
		cloudCfg.AddRunCmd("/install_runner.sh")
		cloudCfg.AddRunCmd("rm -f /install_runner.sh")
		if bootstrapParams.CACertBundle != nil && len(bootstrapParams.CACertBundle) > 0 {
			if err := cloudCfg.AddCACert(bootstrapParams.CACertBundle); err != nil {
				return "", errors.Wrap(err, "adding CA cert bundle")
			}
		}
		var err error
		asStr, err = cloudCfg.Serialize()
		if err != nil {
			return "", errors.Wrap(err, "creating cloud config")
		}
	case params.Windows:
		asStr = string(installScript)
	default:
		return "", fmt.Errorf("unknown os type: %s", bootstrapParams.OSType)
	}

	return asStr, nil
}

func GetTools(osType params.OSType, osArch params.OSArch, tools []*github.RunnerApplicationDownload) (github.RunnerApplicationDownload, error) {
	// Validate image OS. Linux only for now.
	switch osType {
	case params.Linux:
	case params.Windows:
	default:
		return github.RunnerApplicationDownload{}, fmt.Errorf("unsupported OS type: %s", osType)
	}

	switch osArch {
	case params.Amd64:
	case params.Arm:
	case params.Arm64:
	default:
		return github.RunnerApplicationDownload{}, fmt.Errorf("unsupported OS arch: %s", osArch)
	}

	// Find tools for OS/Arch.
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		if tool.OS == nil || tool.Architecture == nil {
			continue
		}

		ghArch, err := ResolveToGithubArch(string(osArch))
		if err != nil {
			continue
		}

		ghOS, err := ResolveToGithubOSType(string(osType))
		if err != nil {
			continue
		}
		if *tool.Architecture == ghArch && *tool.OS == ghOS {
			return *tool, nil
		}
	}
	return github.RunnerApplicationDownload{}, fmt.Errorf("failed to find tools for OS %s and arch %s", osType, osArch)
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

func NewLoggingMiddleware(writer io.Writer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return gorillaHandlers.CombinedLoggingHandler(writer, next)
	}
}

func SanitizeLogEntry(entry string) string {
	return strings.Replace(strings.Replace(entry, "\n", "", -1), "\r", "", -1)
}

func toBase62(uuid []byte) string {
	var i big.Int
	i.SetBytes(uuid[:])
	return i.Text(62)
}

func NewID() string {
	short, err := shortid.Generate()
	if err == nil {
		return toBase62([]byte(short))
	}
	newUUID := uuid.New()
	return toBase62(newUUID[:])
}

func UTF16FromString(s string) ([]uint16, error) {
	buf := make([]uint16, 0, len(s)*2+1)
	for _, r := range s {
		buf = utf16.AppendRune(buf, r)
	}
	return utf16.AppendRune(buf, '\x00'), nil
}

func UTF16ToString(s []uint16) string {
	for i, v := range s {
		if v == 0 {
			s = s[0:i]
			break
		}
	}
	return string(utf16.Decode(s))
}

func Uint16ToByteArray(u []uint16) []byte {
	ret := make([]byte, (len(u)-1)*2)
	for i := 0; i < len(u)-1; i++ {
		binary.LittleEndian.PutUint16(ret[i*2:], uint16(u[i]))
	}
	return ret
}

func UTF16EncodedByteArrayFromString(s string) ([]byte, error) {
	asUint16, err := UTF16FromString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to encode to uint16: %w", err)
	}
	asBytes := Uint16ToByteArray(asUint16)
	return asBytes, nil
}

func CompressData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)

	_, err := gz.Write(data)
	if err != nil {
		return nil, fmt.Errorf("failed to compress data: %w", err)
	}

	if err = gz.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush buffer: %w", err)
	}

	if err = gz.Close(); err != nil {
		return nil, fmt.Errorf("failed to close buffer: %w", err)
	}

	return b.Bytes(), nil
}
