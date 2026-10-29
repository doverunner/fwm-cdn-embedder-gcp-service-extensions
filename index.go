package main

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm/types"

	"github.com/doverunner/fwm-cdn-embedder-gcp-service-extensions/watermarkutil"
	"github.com/doverunner/fwm-cdn-embedder-gcp-service-extensions/wmt"
)

var config watermarkutil.Config

func main() {}

func init() {
	proxywasm.SetPluginContext(func(uint32) types.PluginContext { return &pluginContext{} })
}

type pluginContext struct {
	types.DefaultPluginContext
}

type httpContext struct {
	types.DefaultHttpContext
}

func (*pluginContext) NewHttpContext(uint32) types.HttpContext {
	return &httpContext{}
}

func (*pluginContext) OnPluginStart(int) types.OnPluginStartStatus {
	data, err := proxywasm.GetPluginConfiguration()
	if err != nil {
		proxywasm.LogCriticalf("failed to read the plugin config: %v", err)
		return types.OnPluginStartStatusFailed
	}
	var pluginConfig watermarkutil.Config
	if err := json.Unmarshal(data, &pluginConfig); err != nil {
		proxywasm.LogCriticalf("the plugin config is not the embedder config.json: %v", err)
		return types.OnPluginStartStatusFailed
	}
	watermarkutil.ValidateConfig(pluginConfig)
	config = pluginConfig
	return types.OnPluginStartStatusOK
}

func (*httpContext) OnHttpRequestHeaders(int, bool) types.Action {
	requestPath, err := proxywasm.GetHttpRequestHeader(":path")
	if err != nil {
		proxywasm.LogErrorf("watermark request rejected: no :path header: %v", err)
		return respond(403, "Invalid watermark request")
	}

	uri, querystring, _ := strings.Cut(requestPath, "?")
	finalRequestPath, statusDescription := handler(uri)
	if statusDescription != "" {
		return respond(403, statusDescription)
	}

	if finalRequestPath != uri {
		if !strings.HasPrefix(finalRequestPath, "/") {
			proxywasm.LogErrorf("failed to rewrite :path: %q is not a path", finalRequestPath)
			return respond(500, "Failed to route the watermark request")
		}
		if querystring != "" {
			finalRequestPath += "?" + querystring
		}
		if err := proxywasm.ReplaceHttpRequestHeader(":path", finalRequestPath); err != nil {
			proxywasm.LogErrorf("failed to rewrite :path: %v", err)
			return respond(500, "Failed to route the watermark request")
		}
	}
	return types.ActionContinue
}

func respond(status int, body string) types.Action {
	if err := proxywasm.SendHttpResponse(uint32(status), nil, []byte(body), -1); err != nil {
		proxywasm.LogErrorf("failed to send the response: %v", err)
	}
	return types.ActionPause
}

func isWatermarkedPrefix(requestPrefixFolder string) bool {
	if prefixFolders, ok := config["prefixFolder"].([]any); ok {
		return slices.Contains(prefixFolders, any(requestPrefixFolder))
	}
	return config["prefixFolder"] == requestPrefixFolder
}

func handler(uri string) (finalRequestPath, statusDescription string) {
	proxywasm.LogInfo("request: " + uri)

	defer func() {
		if e := recover(); e != nil {
			proxywasm.LogErrorf("watermark request rejected: %v", e)
			finalRequestPath, statusDescription = "", "Invalid watermark request"
		}
	}()

	finalRequestPath = uri
	uriSegments := strings.Split(uri, "/")
	prefixFolder := uriSegments[1]

	switch {
	case strings.HasPrefix(prefixFolder, "wmt:"):
		finalRequestPath, statusDescription = wmt.AkamaiWMT.GetContentURL(uri, uriSegments, config)
	case isWatermarkedPrefix(prefixFolder):
		uriSegments, hasRevokeToken := watermarkutil.RemoveRevokeToken(uriSegments)
		uri = "/" + strings.Join(uriSegments[1:], "/")
		finalRequestPath, statusDescription = wmt.DoveRunnerAES.GetContentURL(uri, uriSegments, prefixFolder, config, hasRevokeToken)
	}

	if statusDescription == "" {
		proxywasm.LogInfo("response: " + finalRequestPath)
	} else {
		proxywasm.LogInfo("response: 403 " + statusDescription)
	}

	return finalRequestPath, statusDescription
}
