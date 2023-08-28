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

package util

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
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

	runnerErrors "github.com/cloudbase/garm-provider-common/errors"

	commonParams "github.com/cloudbase/garm-provider-common/params"

	"github.com/google/go-github/v54/github"
	"github.com/google/uuid"
	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/pkg/errors"
	"github.com/teris-io/shortid"
	"golang.org/x/crypto/bcrypt"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

const alphanumeric = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// From: https://www.alexedwards.net/blog/validation-snippets-for-go#email-validation
var rxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

var (
	OSToOSTypeMap map[string]commonParams.OSType = map[string]commonParams.OSType{
		"almalinux":  commonParams.Linux,
		"alma":       commonParams.Linux,
		"alpine":     commonParams.Linux,
		"archlinux":  commonParams.Linux,
		"arch":       commonParams.Linux,
		"centos":     commonParams.Linux,
		"ubuntu":     commonParams.Linux,
		"rhel":       commonParams.Linux,
		"suse":       commonParams.Linux,
		"opensuse":   commonParams.Linux,
		"fedora":     commonParams.Linux,
		"debian":     commonParams.Linux,
		"flatcar":    commonParams.Linux,
		"gentoo":     commonParams.Linux,
		"rockylinux": commonParams.Linux,
		"rocky":      commonParams.Linux,
		"windows":    commonParams.Windows,
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
	githubOSTag = map[commonParams.OSType]string{
		commonParams.Linux:   "Linux",
		commonParams.Windows: "Windows",
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
func ResolveToGithubTag(os commonParams.OSType) (string, error) {
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
func GetLoggingWriter(logFile string) (io.Writer, error) {
	var writer io.Writer = os.Stdout
	if logFile != "" {
		dirname := path.Dir(logFile)
		if _, err := os.Stat(dirname); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to create log folder")
			}
			if err := os.MkdirAll(dirname, 0o711); err != nil {
				return nil, fmt.Errorf("failed to create log folder")
			}
		}
		writer = &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    500, // megabytes
			MaxBackups: 3,
			MaxAge:     28,   // days
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

func OSToOSType(os string) (commonParams.OSType, error) {
	osType, ok := OSToOSTypeMap[strings.ToLower(os)]
	if !ok {
		return commonParams.Unknown, fmt.Errorf("no OS to OS type mapping for %s", os)
	}
	return osType, nil
}

func GetTools(osType commonParams.OSType, osArch commonParams.OSArch, tools []*github.RunnerApplicationDownload) (github.RunnerApplicationDownload, error) {
	// Validate image OS. Linux only for now.
	switch osType {
	case commonParams.Linux:
	case commonParams.Windows:
	default:
		return github.RunnerApplicationDownload{}, fmt.Errorf("unsupported OS type: %s", osType)
	}

	switch osArch {
	case commonParams.Amd64:
	case commonParams.Arm:
	case commonParams.Arm64:
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
