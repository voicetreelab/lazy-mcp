package config

import (
	"crypto/tls"
	"errors"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/TBXark/optional-go"
	"github.com/go-sphere/confstore"
	"github.com/go-sphere/confstore/codec"
	"github.com/go-sphere/confstore/provider"
	"github.com/go-sphere/confstore/provider/file"
	"github.com/go-sphere/confstore/provider/http"
)

type StdioMCPClientConfig struct {
	Command string            `json:"command"`
	Env     map[string]string `json:"env"`
	Args    []string          `json:"args"`
}

type SSEMCPClientConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type StreamableMCPClientConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Timeout time.Duration     `json:"timeout"`
}

type MCPClientType string

const (
	MCPClientTypeStdio      MCPClientType = "stdio"
	MCPClientTypeSSE        MCPClientType = "sse"
	MCPClientTypeStreamable MCPClientType = "streamable-http"
)

type MCPServerType string

const (
	MCPServerTypeStdio      MCPServerType = "stdio"
	MCPServerTypeSSE        MCPServerType = "sse"
	MCPServerTypeStreamable MCPServerType = "streamable-http"
)

// ---- V2 ----

type ToolFilterMode string

const (
	ToolFilterModeAllow ToolFilterMode = "allow"
	ToolFilterModeBlock ToolFilterMode = "block"
)

type ToolFilterConfig struct {
	Mode ToolFilterMode `json:"mode,omitempty"`
	List []string       `json:"list,omitempty"`
}

type OptionsV2 struct {
	PanicIfInvalid    optional.Field[bool] `json:"panicIfInvalid,omitempty"`
	LogEnabled        optional.Field[bool] `json:"logEnabled,omitempty"`
	LazyLoad          optional.Field[bool] `json:"lazyLoad,omitempty"`
	RecursiveLazyLoad optional.Field[bool] `json:"recursiveLazyLoad,omitempty"`
	AuthTokens        []string             `json:"authTokens,omitempty"`
	ToolFilter        *ToolFilterConfig    `json:"toolFilter,omitempty"`
}

type MCPProxyConfigV2 struct {
	BaseURL       string        `json:"baseURL"`
	Addr          string        `json:"addr"`
	Name          string        `json:"name"`
	Version       string        `json:"version"`
	Type          MCPServerType `json:"type,omitempty"`
	HierarchyPath string        `json:"hierarchyPath,omitempty"`
	Options       *OptionsV2    `json:"options,omitempty"`
}

type MCPClientConfigV2 struct {
	TransportType MCPClientType `json:"transportType,omitempty"`

	// Stdio
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// SSE or Streamable HTTP
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`

	Options *OptionsV2 `json:"options,omitempty"`
}

func ParseMCPClientConfigV2(conf *MCPClientConfigV2) (any, error) {
	if conf.Command != "" || conf.TransportType == MCPClientTypeStdio {
		if conf.Command == "" {
			return nil, errors.New("command is required for stdio transport")
		}
		return &StdioMCPClientConfig{
			Command: conf.Command,
			Env:     conf.Env,
			Args:    conf.Args,
		}, nil
	}
	if conf.URL != "" {
		if conf.TransportType == MCPClientTypeStreamable {
			return &StreamableMCPClientConfig{
				URL:     conf.URL,
				Headers: conf.Headers,
				Timeout: conf.Timeout,
			}, nil
		} else {
			return &SSEMCPClientConfig{
				URL:     conf.URL,
				Headers: conf.Headers,
			}, nil
		}
	}
	return nil, errors.New("invalid server type")
}

// ---- Config ----

type Config struct {
	McpProxy   *MCPProxyConfigV2             `json:"mcpProxy"`
	McpServers map[string]*MCPClientConfigV2 `json:"mcpServers"`
}

type FullConfig struct {
	DeprecatedServerV1  *MCPProxyConfigV1             `json:"server"`
	DeprecatedClientsV1 map[string]*MCPClientConfigV1 `json:"clients"`

	McpProxy   *MCPProxyConfigV2             `json:"mcpProxy"`
	McpServers map[string]*MCPClientConfigV2 `json:"mcpServers"`
}

func newConfProvider(path string, insecure, expandEnv bool, httpHeaders string, httpTimeout int) (provider.Provider, error) {
	if http.IsRemoteURL(path) {
		var opts []http.Option
		httpClient := nethttp.DefaultClient
		if insecure {
			transport := nethttp.DefaultTransport.(*nethttp.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
			httpClient = &nethttp.Client{Transport: transport}
		}
		if httpTimeout > 0 {
			httpClient.Timeout = time.Duration(httpTimeout) * time.Second
		}
		opts = append(opts, http.WithClient(httpClient))
		if httpHeaders != "" {
			// format: 'Key1:Value1;Key2:Value2'
			headers := make(nethttp.Header)
			for _, kv := range strings.Split(httpHeaders, ";") {
				parts := strings.SplitN(kv, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					if key != "" && value != "" {
						headers.Add(key, value)
					}
				}
			}
		}
		pro := http.New(path, opts...)
		if expandEnv {
			return provider.NewExpandEnv(pro), nil
		} else {
			return pro, nil
		}
	}
	if file.IsLocalPath(path) {
		if expandEnv {
			return provider.NewExpandEnv(file.New(path, file.WithExpandEnv())), nil
		} else {
			return file.New(path), nil
		}
	}
	return nil, errors.New("unsupported config path")
}

func Load(path string, insecure, expandEnv bool, httpHeaders string, httpTimeout int) (*Config, error) {
	pro, err := newConfProvider(path, insecure, expandEnv, httpHeaders, httpTimeout)
	if err != nil {
		return nil, err
	}
	conf, err := confstore.Load[FullConfig](pro, codec.JsonCodec())
	if err != nil {
		return nil, err
	}
	adaptMCPClientConfigV1ToV2(conf)

	if conf.McpProxy == nil {
		return nil, errors.New("mcpProxy is required")
	}
	if conf.McpProxy.Options == nil {
		conf.McpProxy.Options = &OptionsV2{}
	}
	for _, clientConfig := range conf.McpServers {
		if clientConfig.Options == nil {
			clientConfig.Options = &OptionsV2{}
		}
		if clientConfig.Options.AuthTokens == nil {
			clientConfig.Options.AuthTokens = conf.McpProxy.Options.AuthTokens
		}
		if !clientConfig.Options.PanicIfInvalid.Present() {
			clientConfig.Options.PanicIfInvalid = conf.McpProxy.Options.PanicIfInvalid
		}
		if !clientConfig.Options.LogEnabled.Present() {
			clientConfig.Options.LogEnabled = conf.McpProxy.Options.LogEnabled
		}
		if !clientConfig.Options.LazyLoad.Present() {
			clientConfig.Options.LazyLoad = conf.McpProxy.Options.LazyLoad
		}
	}

	if conf.McpProxy.Type == "" {
		conf.McpProxy.Type = MCPServerTypeSSE // default to SSE
	}

	return &Config{
		McpProxy:   conf.McpProxy,
		McpServers: conf.McpServers,
	}, nil
}
