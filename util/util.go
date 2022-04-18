package util

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"runner-manager/config"
	runnerErrors "runner-manager/errors"
)

var (
	OSToOSTypeMap map[string]config.OSType = map[string]config.OSType{
		"ubuntu":  config.Linux,
		"rhel":    config.Linux,
		"centos":  config.Linux,
		"suse":    config.Linux,
		"fedora":  config.Linux,
		"flatcar": config.Linux,
		"windows": config.Windows,
	}
)

// GetLoggingWriter returns a new io.Writer suitable for logging.
func GetLoggingWriter(cfg *config.Config) (io.Writer, error) {
	var writer io.Writer = os.Stdout
	if cfg.LogFile != "" {
		dirname := path.Dir(cfg.LogFile)
		if _, err := os.Stat(dirname); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to create log folder")
			}
			if err := os.MkdirAll(dirname, 0o711); err != nil {
				return nil, fmt.Errorf("failed to create log folder")
			}
		}
		writer = &lumberjack.Logger{
			Filename:   cfg.LogFile,
			MaxSize:    500, // megabytes
			MaxBackups: 3,
			MaxAge:     28,   //days
			Compress:   true, // disabled by default
		}
	}
	return writer, nil
}

func FindRunnerType(runnerType string, runners []config.Runner) (config.Runner, error) {
	for _, runner := range runners {
		if runner.Name == runnerType {
			return runner, nil
		}
	}

	return config.Runner{}, runnerErrors.ErrNotFound
}

func ConvertFileToBase64(file string) (string, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return "", errors.Wrap(err, "reading file")
	}

	return base64.StdEncoding.EncodeToString(bytes), nil
}

// GenerateSSHKeyPair generates a private/public key-pair.
// Shamlessly copied from: https://stackoverflow.com/questions/21151714/go-generate-an-ssh-public-key
func GenerateSSHKeyPair() (pubKey, privKey []byte, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	// generate and write private key as PEM
	var privKeyBuf bytes.Buffer

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return nil, nil, err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	return ssh.MarshalAuthorizedKey(pub), privKeyBuf.Bytes(), nil
}

func OSToOSType(os string) (config.OSType, error) {
	osType, ok := OSToOSTypeMap[strings.ToLower(os)]
	if !ok {
		return config.Unknown, fmt.Errorf("no OS to OS type mapping for %s", os)
	}
	return osType, nil
}
