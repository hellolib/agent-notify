package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hellolib/client-tools/app/larksuite/keychain"
	"github.com/hellolib/client-tools/app/larksuite/output"
	"github.com/hellolib/client-tools/app/larksuite/validate"
)

type Identity string

const (
	AsUser Identity = "user"
	AsBot  Identity = "bot"
)

func (id Identity) IsBot() bool { return id == AsBot }

type LarkBrand string

const (
	BrandFeishu LarkBrand = "feishu"
	BrandLark   LarkBrand = "lark"
)

type Endpoints struct {
	Open     string
	Accounts string
	MCP      string
}

func ResolveEndpoints(brand LarkBrand) Endpoints {
	switch brand {
	case BrandLark:
		return Endpoints{
			Open:     "https://open.larksuite.com",
			Accounts: "https://accounts.larksuite.com",
			MCP:      "https://mcp.larksuite.com",
		}
	default:
		return Endpoints{
			Open:     "https://open.feishu.cn",
			Accounts: "https://accounts.feishu.cn",
			MCP:      "https://mcp.feishu.cn",
		}
	}
}

func ResolveOpenBaseURL(brand LarkBrand) string {
	return ResolveEndpoints(brand).Open
}

type ConfigError struct {
	Code    int
	Type    string
	Message string
	Hint    string
}

func (e *ConfigError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s\n  %s", e.Message, e.Hint)
	}
	return e.Message
}

type SecretRef struct {
	Source   string `json:"source"`
	Provider string `json:"provider,omitempty"`
	ID       string `json:"id"`
}

type SecretInput struct {
	Plain string
	Ref   *SecretRef
}

func PlainSecret(s string) SecretInput {
	return SecretInput{Plain: s}
}

func (s SecretInput) IsZero() bool {
	return s.Plain == "" && s.Ref == nil
}

func (s SecretInput) IsSecretRef() bool {
	return s.Ref != nil
}

func (s SecretInput) IsPlain() bool {
	return s.Ref == nil
}

func (s SecretInput) MarshalJSON() ([]byte, error) {
	if s.Ref != nil {
		return json.Marshal(s.Ref)
	}
	return json.Marshal(s.Plain)
}

func (s *SecretInput) UnmarshalJSON(data []byte) error {
	var plain string
	if err := json.Unmarshal(data, &plain); err == nil {
		s.Plain = plain
		s.Ref = nil
		return nil
	}

	var ref SecretRef
	if err := json.Unmarshal(data, &ref); err == nil && isValidSource(ref.Source) && ref.ID != "" {
		s.Ref = &ref
		s.Plain = ""
		return nil
	}
	return fmt.Errorf("appSecret must be a string or {source, id} object")
}

var ValidSecretSources = map[string]bool{
	"file": true, "keychain": true,
}

func isValidSource(source string) bool {
	return ValidSecretSources[source]
}

type AppUser struct {
	UserOpenId string `json:"userOpenId"`
	UserName   string `json:"userName"`
}

type AppConfig struct {
	AppId     string      `json:"appId"`
	AppSecret SecretInput `json:"appSecret"`
	Brand     LarkBrand   `json:"brand"`
	Lang      string      `json:"lang,omitempty"`
	DefaultAs string      `json:"defaultAs,omitempty"`
	Users     []AppUser   `json:"users"`
}

type MultiAppConfig struct {
	Apps []AppConfig `json:"apps"`
}

type CliConfig struct {
	AppID      string
	AppSecret  string
	Brand      LarkBrand
	DefaultAs  string
	UserOpenId string
	UserName   string
}

func GetConfigDir() string {
	if dir := os.Getenv("LARKSUITE_CLI_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		fmt.Fprintf(os.Stderr, "warning: unable to determine home directory: %v\n", err)
	}
	return filepath.Join(home, ".lark-cli")
}

func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.json")
}

func LoadMultiAppConfig() (*MultiAppConfig, error) {
	data, err := os.ReadFile(GetConfigPath())
	if err != nil {
		return nil, err
	}

	var multi MultiAppConfig
	if err := json.Unmarshal(data, &multi); err != nil {
		return nil, fmt.Errorf("invalid config format: %w", err)
	}
	if len(multi.Apps) == 0 {
		return nil, fmt.Errorf("invalid config format: no apps")
	}
	return &multi, nil
}

func SaveMultiAppConfig(config *MultiAppConfig) error {
	dir := GetConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return validate.AtomicWrite(GetConfigPath(), append(data, '\n'), 0600)
}

func RequireConfig(kc keychain.KeychainAccess) (*CliConfig, error) {
	raw, err := LoadMultiAppConfig()
	if err != nil || raw == nil || len(raw.Apps) == 0 {
		return nil, &ConfigError{Code: 2, Type: "config", Message: "not configured", Hint: "run `lark-cli config init --new` in the background. It blocks and outputs a verification URL — retrieve the URL and open it in a browser to complete setup."}
	}
	app := raw.Apps[0]
	secret, err := ResolveSecretInput(app.AppSecret, kc)
	if err != nil {
		var exitErr *output.ExitError
		if errors.As(err, &exitErr) {
			return nil, exitErr
		}
		return nil, &ConfigError{Code: 2, Type: "config", Message: err.Error()}
	}
	cfg := &CliConfig{
		AppID:     app.AppId,
		AppSecret: secret,
		Brand:     app.Brand,
		DefaultAs: app.DefaultAs,
	}
	if len(app.Users) > 0 {
		cfg.UserOpenId = app.Users[0].UserOpenId
		cfg.UserName = app.Users[0].UserName
	}
	return cfg, nil
}

const secretKeyPrefix = "appsecret:"

func secretAccountKey(appId string) string {
	return secretKeyPrefix + appId
}

func ResolveSecretInput(s SecretInput, kc keychain.KeychainAccess) (string, error) {
	if s.Ref == nil {
		return s.Plain, nil
	}
	switch s.Ref.Source {
	case "file":
		data, err := os.ReadFile(s.Ref.ID)
		if err != nil {
			return "", fmt.Errorf("failed to read secret file %s: %w", s.Ref.ID, err)
		}
		return strings.TrimSpace(string(data)), nil
	case "keychain":
		return kc.Get(keychain.LarkCliService, s.Ref.ID)
	default:
		return "", fmt.Errorf("unknown secret source: %s", s.Ref.Source)
	}
}

func ForStorage(appId string, input SecretInput, kc keychain.KeychainAccess) (SecretInput, error) {
	if !input.IsPlain() {
		return input, nil
	}
	key := secretAccountKey(appId)
	if err := kc.Set(keychain.LarkCliService, key, input.Plain); err != nil {
		return SecretInput{}, fmt.Errorf("keychain unavailable: %w\nhint: use file: reference in config to bypass keychain", err)
	}
	return SecretInput{Ref: &SecretRef{Source: "keychain", ID: key}}, nil
}

func RemoveSecretStore(input SecretInput, kc keychain.KeychainAccess) {
	if input.IsSecretRef() && input.Ref.Source == "keychain" {
		_ = kc.Remove(keychain.LarkCliService, input.Ref.ID)
	}
}
