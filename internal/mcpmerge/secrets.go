package mcpmerge

import (
	"fmt"
	"strings"
)

// sensitiveFieldFragments are substrings (case-insensitive) that, when found
// in an MCP server field key, indicate the value must be an env-reference
// or list-of-env-var-names — never a literal secret. Source registries are
// git-committed and often team-shared; literal secrets would be a critical
// security failure.
//
// Conservative on purpose: false positives (rejecting a valid non-secret
// field) are far less harmful than false negatives (allowing a leaked
// secret). Users hit by a false positive can rename the field.
var sensitiveFieldFragments = []string{
	"token",
	"secret",
	"password",
	"passwd",
	"api_key",
	"apikey",
	"private_key",
	"privatekey",
	"bearer",
	"_key", // catches things like access_key, signing_key; pairs with !contains("public")
}

// ValidateNoLiteralSecrets walks an MCP server's parsed body and ensures no
// sensitive-named field contains a literal string value. Returns nil if
// safe, descriptive error if a likely literal secret is found.
//
// What's allowed for sensitive fields:
//   - env_vars = ["VAR_NAME"]      ← Codex's pattern; value never inlined
//   - bearer_token_env_var = "VAR" ← Codex HTTP server pattern; same intent
//   - value referencing an env var: starts with "$" or "${" — e.g., "$TOKEN", "${env:TOKEN}"
//   - non-string values (rare — usually ints/bools for non-secret config)
//
// What's rejected:
//   - sensitive-named field with a literal string value that isn't an env reference
func ValidateNoLiteralSecrets(serverName string, body map[string]interface{}) error {
	for key, val := range body {
		if !isSensitiveFieldName(key) {
			continue
		}
		if violation := checkSensitiveValue(key, val); violation != "" {
			return fmt.Errorf("[mcp_servers.%s].%s: %s — use env-reference form (e.g., env_vars = [\"%s\"] for Codex, or \"$VAR_NAME\" / \"${env:VAR_NAME}\" for inline)", serverName, key, violation, strings.ToUpper(key))
		}
	}
	// Also walk nested env maps explicitly — common pattern is env = { TOKEN = "..." }.
	if envRaw, ok := body["env"]; ok {
		if envMap, ok := envRaw.(map[string]interface{}); ok {
			for envKey, envVal := range envMap {
				if !isSensitiveFieldName(envKey) {
					continue
				}
				if violation := checkSensitiveValue(envKey, envVal); violation != "" {
					return fmt.Errorf("[mcp_servers.%s].env.%s: %s — use \"$%s\" or \"${env:%s}\" to forward the value from the user's environment instead of inlining it", serverName, envKey, violation, strings.ToUpper(envKey), strings.ToUpper(envKey))
				}
			}
		}
	}
	return nil
}

func isSensitiveFieldName(key string) bool {
	lower := strings.ToLower(key)
	// Exempt: fields whose name explicitly identifies an env-var name
	// (the value IS the env var name, not the secret). Codex uses this:
	// `bearer_token_env_var = "GH_PAT"` means "look up GH_PAT in env."
	if strings.HasSuffix(lower, "_env_var") || strings.HasSuffix(lower, "_env") || strings.HasSuffix(lower, "_envvar") {
		return false
	}
	// Exempt: public_key (the _key heuristic shouldn't catch public keys).
	if strings.Contains(lower, "public_key") || strings.Contains(lower, "publickey") {
		return false
	}
	for _, frag := range sensitiveFieldFragments {
		if strings.Contains(lower, frag) {
			return true
		}
	}
	return false
}

func checkSensitiveValue(_ string, val interface{}) string {
	// Non-string values are not a literal-secret concern.
	str, ok := val.(string)
	if !ok {
		return ""
	}
	if str == "" {
		return ""
	}
	// Env-reference forms are safe.
	if strings.HasPrefix(str, "$") {
		return ""
	}
	return fmt.Sprintf("contains a literal string value (%d characters)", len(str))
}
