package transformation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// PinPath is the script every fetch of the model sources its pin from.
const PinPath = "scripts/sysml-v1tov2-pin.sh"

// RequireEnv turns an absent pinned model into a failure, as CI sets it.
const RequireEnv = "OPENSYSML_REQUIRE_SYSML_V1TOV2"

// ModelDir is the directory the downloader provisions the model into.
const ModelDir = "build/sysml-v1tov2"

// Pin is the model's identity as scripts/sysml-v1tov2-pin.sh records it.
type Pin struct {
	Document string
	Version  string
	URL      string
	SHA256   string
	File     string
}

// The script's variables; an environment value overrides each for the downloader,
// so it overrides here too, or the two would fetch and verify different pins.
const (
	DocumentEnv = "SYSML_V1TOV2_DOCUMENT"
	VersionEnv  = "SYSML_V1TOV2_VERSION"
	URLEnv      = "SYSML_V1TOV2_URL"
	SHA256Env   = "SYSML_V1TOV2_SHA256"
)

var (
	pinDocumentRe = regexp.MustCompile(`SYSML_V1TOV2_DOCUMENT="\$\{SYSML_V1TOV2_DOCUMENT:-([^}"]+)\}"`)
	pinVersionRe  = regexp.MustCompile(`SYSML_V1TOV2_VERSION="\$\{SYSML_V1TOV2_VERSION:-([^}"]+)\}"`)
	pinURLRe      = regexp.MustCompile(`SYSML_V1TOV2_URL="\$\{SYSML_V1TOV2_URL:-([^}"]+)\}"`)
	pinSHA256Re   = regexp.MustCompile(`SYSML_V1TOV2_SHA256="\$\{SYSML_V1TOV2_SHA256:-([0-9a-f]{64})\}"`)
	pinFileRe     = regexp.MustCompile(`SYSML_V1TOV2_FILE="([^"]+)"`)
	sha256Re      = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// ReadPin resolves the pin as scripts/sysml-v1tov2-pin.sh does: the script's
// defaults under the repository root, each replaced by its environment variable
// when set.
func ReadPin(repo string) (Pin, error) {
	pin, err := readPinDefaults(repo)
	if err != nil {
		return Pin{}, err
	}
	for _, o := range []struct {
		env   string
		field *string
	}{{DocumentEnv, &pin.Document}, {VersionEnv, &pin.Version}, {URLEnv, &pin.URL}, {SHA256Env, &pin.SHA256}} {
		if v, ok := os.LookupEnv(o.env); ok && v != "" {
			*o.field = v
		}
	}
	if !sha256Re.MatchString(pin.SHA256) {
		return Pin{}, fmt.Errorf("%s=%q is not a lowercase hex sha256", SHA256Env, pin.SHA256)
	}
	return pin, nil
}

func readPinDefaults(repo string) (Pin, error) {
	path := filepath.Join(repo, filepath.FromSlash(PinPath))
	content, err := os.ReadFile(path) // #nosec G304 -- the pin is at a fixed path in this repository
	if err != nil {
		return Pin{}, fmt.Errorf("read %s: %w", PinPath, err)
	}
	doc := pinDocumentRe.FindSubmatch(content)
	version := pinVersionRe.FindSubmatch(content)
	url := pinURLRe.FindSubmatch(content)
	sum := pinSHA256Re.FindSubmatch(content)
	file := pinFileRe.FindSubmatch(content)
	if doc == nil || version == nil || url == nil || sum == nil || file == nil {
		return Pin{}, fmt.Errorf("%s pins no SYSML_V1TOV2_DOCUMENT/SYSML_V1TOV2_VERSION/SYSML_V1TOV2_URL/SYSML_V1TOV2_SHA256/SYSML_V1TOV2_FILE", PinPath)
	}
	return Pin{Document: string(doc[1]), Version: string(version[1]), URL: string(url[1]), SHA256: string(sum[1]), File: string(file[1])}, nil
}

// ModelPath is where the pin's file is provisioned, unless -xmi overrides it.
func modelPath(root, pinFile, given string) string {
	if given != "" {
		return given
	}
	return filepath.Join(root, filepath.FromSlash(ModelDir), pinFile)
}

// Digest is the sha256 of the model file at path, as the pin spells it.
func Digest(path string) (string, error) {
	content, err := os.ReadFile(path) // #nosec G304 -- callers name the located model file
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}
