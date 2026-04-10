package templates

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"text/template"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/cache"
	"github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/util/x509"
)

//go:embed all:userdata
var Userdata embed.FS

type WrapperContext struct {
	CallbackToken string
	MetadataURL   string
	CACertBundle  string
}

// InstallContext wraps the vendored InstallRunnerParams with agent-specific
// fields so templates can render agent values directly without requiring
// the runner to fetch metadata and parse it with jq at runtime.
type InstallContext struct {
	cloudconfig.InstallRunnerParams
	AgentMode        bool
	AgentDownloadURL string
	AgentURL         string
	AgentToken       string
	AgentShell       string // "true" or "false", used verbatim in TOML config
}

func GetTemplateContent(osType commonParams.OSType, forge params.EndpointType) ([]byte, error) {
	switch forge {
	case params.GithubEndpointType, params.GiteaEndpointType:
		switch osType {
		case commonParams.Linux, commonParams.Windows:
		default:
			return nil, runnerErrors.NewNotFoundError("could not find template for forge %s and OS type: %q", forge, osType)
		}
	default:
		return nil, runnerErrors.NewNotFoundError("could not find template for forge type: %q", forge)
	}

	templateName := fmt.Sprintf("%s_%s_userdata.tmpl", forge, osType)
	fd, err := Userdata.Open(path.Join("userdata", templateName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, runnerErrors.NewNotFoundError("could not find template for OS type %q and forge %q", osType, forge)
		}
	}

	data, err := io.ReadAll(fd)
	if err != nil {
		fd.Close()
		return nil, fmt.Errorf("failed to read template: %w", err)
	}
	fd.Close()
	return data, nil
}

func RenderRunnerInstallScript(tpl string, context any) ([]byte, error) {
	t, err := template.New("").Parse(tpl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, context); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	data := buf.String()
	return []byte(data), nil
}

func RenderRunnerInstallWrapper(ctx context.Context, osType commonParams.OSType, metadataURL, token string) ([]byte, error) {
	tmpl, err := template.ParseFS(Userdata, "userdata/*_wrapper.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	templateCtx := WrapperContext{
		MetadataURL:   metadataURL,
		CallbackToken: token,
	}

	controllerInfo := cache.ControllerInfo()
	if osType == commonParams.Windows && len(controllerInfo.CACertBundle) > 0 {
		// For linux, we combine the endpoint CA and controller CA, deduplicate and set it
		// as cloud-config extra CA certificates. So by the time the wrapper runs, we already
		// have the CA installed.
		// The only edge case left is the one that when users overwrite runner install templates
		// or when no garm managed template is set on the pool or scale set, windows userdata is managed
		// by the built-in provider template, which may not explicitly handle the controller CA before
		// making calls to the GARM API.
		asMap, err := x509.RawCABundleToMap(controllerInfo.CACertBundle)
		if err == nil {
			asJs, err := json.Marshal(asMap)
			if err != nil {
				slog.ErrorContext(ctx, "failed to marshal controller CA cert bundle", "error", err)
			} else {
				templateCtx.CACertBundle = string(asJs)
			}
		} else {
			slog.ErrorContext(ctx, "failed to convert CA bundle to map", "error", err)
		}
	}

	templateName := fmt.Sprintf("%s_wrapper.tmpl", osType)
	var b bytes.Buffer
	wr := bufio.NewWriter(&b)
	wr.Flush()

	if err := tmpl.ExecuteTemplate(wr, templateName, templateCtx); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	wr.Flush()

	return b.Bytes(), nil
}
