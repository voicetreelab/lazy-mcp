package main

import (
	"encoding/json"
	"errors"
)

// ---- V1 ----

type MCPClientConfigV1 struct {
	Type           MCPClientType   `json:"type"`
	Config         json.RawMessage `json:"config"`
	PanicIfInvalid bool            `json:"panicIfInvalid"`
	LogEnabled     bool            `json:"logEnabled"`
	AuthTokens     []string        `json:"authTokens"`
}

type MCPProxyConfigV1 struct {
	BaseURL          string   `json:"baseURL"`
	Addr             string   `json:"addr"`
	Name             string   `json:"name"`
	Version          string   `json:"version"`
	GlobalAuthTokens []string `json:"globalAuthTokens"`
}

func parseMCPClientConfigV1(conf *MCPClientConfigV1) (any, error) {
	switch conf.Type {
	case MCPClientTypeStdio:
		var config StdioMCPClientConfig
		err := json.Unmarshal(conf.Config, &config)
		if err != nil {
			return nil, err
		}
		return &config, nil
	case MCPClientTypeSSE:
		var config SSEMCPClientConfig
		err := json.Unmarshal(conf.Config, &config)
		if err != nil {
			return nil, err
		}
		return &config, nil
	case MCPClientTypeStreamable:
		var config StreamableMCPClientConfig
		err := json.Unmarshal(conf.Config, &config)
		if err != nil {
			return nil, err
		}
		return &config, nil
	default:
		return nil, errors.New("invalid client type")
	}
}

func adaptMCPClientConfigV1ToV2(conf *FullConfig) {
	if conf.DeprecatedServerV1 != nil && conf.McpProxy == nil {
		v1 := conf.DeprecatedServerV1
		conf.McpProxy = &MCPProxyConfigV2{
			BaseURL: v1.BaseURL,
			Addr:    v1.Addr,
			Name:    v1.Name,
			Version: v1.Version,
			Options: &OptionsV2{
				AuthTokens: v1.GlobalAuthTokens,
			},
		}
	}

	if len(conf.DeprecatedClientsV1) > 0 && len(conf.McpServers) == 0 {
		conf.McpServers = make(map[string]*MCPClientConfigV2)
		for name, clientConfig := range conf.DeprecatedClientsV1 {
			clientInfo, cErr := parseMCPClientConfigV1(clientConfig)
			if cErr != nil {
				continue
			}
			options := &OptionsV2{
				AuthTokens: clientConfig.AuthTokens,
			}
			if conf.DeprecatedServerV1 != nil && len(conf.DeprecatedServerV1.GlobalAuthTokens) > 0 {
				options.AuthTokens = append(options.AuthTokens, conf.DeprecatedServerV1.GlobalAuthTokens...)
			}
			switch v := clientInfo.(type) {
			case *StdioMCPClientConfig:
				conf.McpServers[name] = &MCPClientConfigV2{
					Command: v.Command,
					Args:    v.Args,
					Env:     v.Env,
					Options: options,
				}
			case *SSEMCPClientConfig:
				conf.McpServers[name] = &MCPClientConfigV2{
					URL:     v.URL,
					Headers: v.Headers,
					Options: options,
				}
			case *StreamableMCPClientConfig:
				conf.McpServers[name] = &MCPClientConfigV2{
					URL:     v.URL,
					Headers: v.Headers,
					Timeout: v.Timeout,
					Options: options,
				}
			default:
				continue
			}
		}
	}
	// remove deprecated fields
	conf.DeprecatedServerV1 = nil
	conf.DeprecatedClientsV1 = nil
}
