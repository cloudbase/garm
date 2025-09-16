package templates

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path"
	"text/template"

	"github.com/cloudbase/garm-provider-common/cloudconfig"
	runnerErrors "github.com/cloudbase/garm-provider-common/errors"
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
)

//go:embed all:userdata
var Userdata embed.FS

type WrapperContext struct {
	CallbackToken string
	MetadataURL   string
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

func RenderRunnerInstallScript(tpl string, context cloudconfig.InstallRunnerParams) ([]byte, error) {
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

func RenderRunnerInstallWrapper(osType commonParams.OSType, metadataURL, token string) ([]byte, error) {
	tmpl, err := template.ParseFS(Userdata, "userdata/*_wrapper.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	templateCtx := WrapperContext{
		MetadataURL:   metadataURL,
		CallbackToken: token,
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
