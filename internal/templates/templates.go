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
	"regexp"
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
	CallbackURL   string
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
	// ForceInsecureGARMAgent allows the agent to connect back to GARM even when
	// TLS is not enabled in GARM itself.
	ForceInsecureGARMAgent bool
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

// parseTemplate is the single entrypoint used to parse a runner install
// template. Both rendering and validation go through it so that a template
// which validates successfully is guaranteed to parse identically at render
// time (same delimiters, same absence of a custom FuncMap). If the render path
// ever gains custom functions or options, adding them here keeps validation
// faithful automatically.
func parseTemplate(tpl string) (*template.Template, error) {
	return template.New("").Parse(tpl)
}

// tmplParseErrRe matches the prefix text/template adds to parse errors, e.g.
// `template: :3: unexpected "and"`. Our templates are unnamed, so the name
// segment between the two colons is empty.
var tmplParseErrRe = regexp.MustCompile(`^template: [^:]*:(\d+): *(.*)$`)

// cleanTemplateParseError rewrites the noisy `template: :<line>: <msg>` prefix
// that text/template emits into a friendlier `line <n>: <msg>` form while
// preserving the line number so callers (e.g. the web UI editor) can anchor a
// diagnostic to it. Errors that don't match the expected shape are returned
// verbatim.
func cleanTemplateParseError(err error) string {
	msg := err.Error()
	if m := tmplParseErrRe.FindStringSubmatch(msg); m != nil {
		return fmt.Sprintf("line %s: %s", m[1], m[2])
	}
	return msg
}

// ValidateTemplate performs parse-time validation of a runner install template.
//
// It only parses the template; it deliberately does NOT execute it. Parsing
// validates the grammar (delimiter balance, pipelines, block/end matching and
// references to known functions) without ever inspecting the render context, so
// it is completely independent of the variable data a template is rendered
// against — most notably the per-pool ExtraSpecs/ExtraContext map. A reference
// such as `{{ index .ExtraContext "gitea_cache_address" }}` is valid regardless
// of which keys exist at render time. Executing to validate would be both
// unsound (the context is unknown and variable at save time) and unnecessary
// (a missing map key yields "" rather than an error), which is why parse-time
// is exactly the right level.
func ValidateTemplate(data []byte) error {
	// Return typed bad-request errors (not plain fmt.Errorf) so that callers
	// which surface this directly get an HTTP 400 with the message in the
	// response body. A plain error would fall through handleError's default
	// case to an opaque 500 with the details stripped.
	if len(data) == 0 {
		return runnerErrors.NewBadRequestError("template data is empty")
	}
	if _, err := parseTemplate(string(data)); err != nil {
		return runnerErrors.NewBadRequestError("invalid template: %s", cleanTemplateParseError(err))
	}
	return nil
}

func RenderRunnerInstallScript(tpl string, context any) ([]byte, error) {
	t, err := parseTemplate(tpl)
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

func RenderRunnerInstallWrapper(ctx context.Context, osType commonParams.OSType, metadataURL, callbackURL, token string) ([]byte, error) {
	tmpl, err := template.ParseFS(Userdata, "userdata/*_wrapper.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	templateCtx := WrapperContext{
		MetadataURL:   metadataURL,
		CallbackURL:   callbackURL,
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
