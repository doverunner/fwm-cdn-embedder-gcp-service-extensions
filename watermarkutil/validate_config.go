package watermarkutil

import (
	"fmt"
	"slices"
	"strings"

	"github.com/proxy-wasm/proxy-wasm-go-sdk/proxywasm"
)

var composingTypes = []string{"unlabeled_a_variant", "directory_prefix", "aws"}

const aesKeyLength = 32

type Config map[string]any

func ValidateConfig(config Config) {
	var problems []string

	if !slices.Contains(composingTypes, strings.ToLower(fmt.Sprint(config["type"]))) {
		problems = append(problems, fmt.Sprintf("type must be one of %s, not '%v'", strings.Join(composingTypes, ", "), config["type"]))
	}

	if aesKey, _ := config["aesKey"].(string); len(aesKey) != aesKeyLength {
		problems = append(problems, fmt.Sprintf("aesKey must be %d characters; it is not the site key yet", aesKeyLength))
	}

	if _, ok := config["availableInterval"].(float64); !ok {
		problems = append(problems, "availableInterval must be a number of minutes, or 0 to disable the check")
	}

	prefixFolders, ok := config["prefixFolder"].([]any)
	if !ok {
		prefixFolders = []any{config["prefixFolder"]}
	}
	for _, folder := range prefixFolders {
		if strings.Contains(fmt.Sprint(folder), "/") {
			problems = append(problems, fmt.Sprintf("prefixFolder '%v' must be a single path segment, without a slash", folder))
		}
	}

	if len(problems) > 0 {
		proxywasm.LogError("watermark embedder configuration is unusable: " + strings.Join(problems, "; "))
	}
}
