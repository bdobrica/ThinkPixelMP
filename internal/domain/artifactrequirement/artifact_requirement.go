package artifactrequirement

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

const (
	SchemaVersion              = 1
	MaxNormalizedMetadataBytes = 1 << 20
)

var (
	dnsTokenPattern   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
	capabilityPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9_-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9_-]*[a-z0-9])?)+$`)
	positiveInteger   = regexp.MustCompile(`^[1-9][0-9]*$`)
)

type ArtifactRequirement struct {
	tenantID              shared.UUID
	artifactVersionID     shared.UUID
	requirementDigest     shared.Digest
	normalizedRequirement []byte
}

// Repository is the tenant-scoped persistence boundary for requirements. Its
// operations join a transaction carried by context when supported.
type Repository interface {
	Create(context.Context, ArtifactRequirement) error
	Get(context.Context, shared.UUID, shared.UUID) (ArtifactRequirement, error)
}

// New accepts normalized requirement bytes produced by a trusted descriptor
// validator. It validates the closed V1 requirement vocabulary and binds the
// supplied digest to those exact bytes. Requirements remain declarations only.
func New(tenantID, artifactVersionID shared.UUID, requirementDigest shared.Digest, normalizedRequirement []byte) (ArtifactRequirement, error) {
	return restore(tenantID, artifactVersionID, requirementDigest, normalizedRequirement)
}

// Restore validates an ArtifactRequirement loaded from an adapter.
func Restore(tenantID, artifactVersionID shared.UUID, requirementDigest shared.Digest, normalizedRequirement []byte) (ArtifactRequirement, error) {
	return restore(tenantID, artifactVersionID, requirementDigest, normalizedRequirement)
}

func restore(tenantID, artifactVersionID shared.UUID, requirementDigest shared.Digest, normalizedRequirement []byte) (ArtifactRequirement, error) {
	for label, identifier := range map[string]shared.UUID{"tenant ID": tenantID, "ArtifactVersion ID": artifactVersionID} {
		if _, err := identifier.MarshalText(); err != nil {
			return ArtifactRequirement{}, fmt.Errorf("artifact requirement: %s: %w", label, err)
		}
	}
	if _, err := requirementDigest.MarshalText(); err != nil {
		return ArtifactRequirement{}, fmt.Errorf("artifact requirement: digest: %w", err)
	}
	if len(normalizedRequirement) < 2 || len(normalizedRequirement) > MaxNormalizedMetadataBytes {
		return ArtifactRequirement{}, fmt.Errorf("artifact requirement: normalized metadata exceeds bounds")
	}
	if !utf8.Valid(normalizedRequirement) || !json.Valid(normalizedRequirement) || hasDuplicateKeyOrTrailingContent(normalizedRequirement) {
		return ArtifactRequirement{}, fmt.Errorf("artifact requirement: invalid normalized metadata")
	}
	if shared.SHA256Digest(normalizedRequirement) != requirementDigest {
		return ArtifactRequirement{}, fmt.Errorf("artifact requirement: digest does not match normalized metadata")
	}
	if err := validateDocument(normalizedRequirement); err != nil {
		return ArtifactRequirement{}, fmt.Errorf("artifact requirement: invalid V1 document: %w", err)
	}
	metadataCopy := append([]byte(nil), normalizedRequirement...)
	return ArtifactRequirement{tenantID, artifactVersionID, requirementDigest, metadataCopy}, nil
}

func validateDocument(value []byte) error {
	document, err := decodeObject(value, []string{"schema_version", "capabilities", "runtime", "network", "integrations"}, []string{"schema_version"})
	if err != nil || !exactInteger(document["schema_version"], SchemaVersion) {
		return fmt.Errorf("schema version")
	}
	if raw, ok := document["capabilities"]; ok {
		if err := validateCapabilities(raw); err != nil {
			return err
		}
	}
	if raw, ok := document["runtime"]; ok {
		if err := validateRuntime(raw); err != nil {
			return err
		}
	}
	if raw, ok := document["network"]; ok {
		if err := validateNetwork(raw); err != nil {
			return err
		}
	}
	if raw, ok := document["integrations"]; ok {
		if err := validateIntegrations(raw); err != nil {
			return err
		}
	}
	return nil
}

func validateCapabilities(raw json.RawMessage) error {
	value, err := decodeObject(raw, []string{"required", "optional"}, nil)
	if err != nil {
		return fmt.Errorf("capabilities")
	}
	for _, field := range []string{"required", "optional"} {
		if list, ok := value[field]; ok {
			if _, err := stringSet(list, 0, 256, capabilityPattern.MatchString); err != nil {
				return fmt.Errorf("capabilities %s", field)
			}
		}
	}
	return nil
}

func validateRuntime(raw json.RawMessage) error {
	fields := []string{"profile", "minimum_isolation_class", "os", "architectures", "minimum_cpu_millicores", "minimum_memory_bytes", "minimum_ephemeral_storage_bytes", "minimum_workspace_storage_bytes", "durable_workspace_required", "gpu_required", "gpu_classes", "adapter"}
	value, err := decodeObject(raw, fields, nil)
	if err != nil {
		return fmt.Errorf("runtime")
	}
	if raw, ok := value["profile"]; ok && !validJSONString(raw, 1, 63, dnsTokenPattern.MatchString) {
		return fmt.Errorf("runtime profile")
	}
	if raw, ok := value["minimum_isolation_class"]; ok && !stringEnum(raw, "container-standard", "microvm-strong", "confidential-strong") {
		return fmt.Errorf("runtime isolation")
	}
	if raw, ok := value["os"]; ok && !stringEnum(raw, "linux") {
		return fmt.Errorf("runtime os")
	}
	if raw, ok := value["architectures"]; ok {
		if _, err := stringSet(raw, 1, 4, func(item string) bool { return item == "amd64" || item == "arm64" }); err != nil {
			return fmt.Errorf("runtime architectures")
		}
	}
	for _, field := range []string{"minimum_cpu_millicores", "minimum_memory_bytes", "minimum_ephemeral_storage_bytes", "minimum_workspace_storage_bytes"} {
		if raw, ok := value[field]; ok && !positiveInteger.MatchString(strings.TrimSpace(string(raw))) {
			return fmt.Errorf("runtime quantity")
		}
	}
	for _, field := range []string{"durable_workspace_required", "gpu_required"} {
		if raw, ok := value[field]; ok && !validBoolean(raw) {
			return fmt.Errorf("runtime boolean")
		}
	}
	if raw, ok := value["gpu_classes"]; ok {
		if _, err := stringSet(raw, 0, 8, dnsTokenPattern.MatchString); err != nil {
			return fmt.Errorf("runtime gpu classes")
		}
	}
	if raw, ok := value["adapter"]; ok {
		adapter, err := decodeObject(raw, []string{"kind", "compatibility"}, []string{"kind", "compatibility"})
		if err != nil || !validJSONString(adapter["kind"], 1, 63, dnsTokenPattern.MatchString) || !validJSONString(adapter["compatibility"], 1, 128, func(string) bool { return true }) {
			return fmt.Errorf("runtime adapter")
		}
	}
	return nil
}

func validateNetwork(raw json.RawMessage) error {
	value, err := decodeObject(raw, []string{"profile", "endpoint_classes"}, []string{"profile"})
	if err != nil || !stringEnum(value["profile"], "none", "thinkpixel-only", "package-mirrors", "restricted-external", "unrestricted-standalone") {
		return fmt.Errorf("network profile")
	}
	profile, _ := jsonString(value["profile"])
	classes, present := value["endpoint_classes"]
	if profile == "restricted-external" {
		if !present {
			return fmt.Errorf("network endpoint classes")
		}
		if _, err := stringSet(classes, 1, 32, dnsTokenPattern.MatchString); err != nil {
			return fmt.Errorf("network endpoint classes")
		}
		return nil
	}
	if present {
		values, err := stringSet(classes, 0, 0, dnsTokenPattern.MatchString)
		if err != nil || len(values) != 0 {
			return fmt.Errorf("network endpoint classes")
		}
	}
	return nil
}

func validateIntegrations(raw json.RawMessage) error {
	items, err := rawArray(raw, 0, 64)
	if err != nil {
		return fmt.Errorf("integrations")
	}
	for _, rawItem := range items {
		item, err := decodeObject(rawItem, []string{"kind", "identifier", "required"}, []string{"kind", "identifier", "required"})
		if err != nil || !stringEnum(item["kind"], "mcp-server", "a2a-peer", "model-feature", "model-family", "artifact-store") || !validJSONString(item["identifier"], 1, 255, func(string) bool { return true }) || !validBoolean(item["required"]) {
			return fmt.Errorf("integration")
		}
	}
	return nil
}

func decodeObject(raw []byte, allowed, required []string) (map[string]json.RawMessage, error) {
	if jsonKind(raw) != '{' {
		return nil, fmt.Errorf("expected object")
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedSet[field] = struct{}{}
	}
	for field := range value {
		if _, ok := allowedSet[field]; !ok {
			return nil, fmt.Errorf("unknown field")
		}
	}
	for _, field := range required {
		if _, ok := value[field]; !ok {
			return nil, fmt.Errorf("missing field")
		}
	}
	return value, nil
}

func rawArray(raw []byte, minimum, maximum int) ([]json.RawMessage, error) {
	if jsonKind(raw) != '[' {
		return nil, fmt.Errorf("expected array")
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || len(values) < minimum || len(values) > maximum {
		return nil, fmt.Errorf("invalid array")
	}
	return values, nil
}

func stringSet(raw []byte, minimum, maximum int, valid func(string) bool) ([]string, error) {
	items, err := rawArray(raw, minimum, maximum)
	if err != nil {
		return nil, err
	}
	values := make([]string, len(items))
	for index, item := range items {
		value, ok := jsonString(item)
		if !ok || !valid(value) {
			return nil, fmt.Errorf("invalid string set")
		}
		values[index] = value
	}
	if !sort.StringsAreSorted(values) {
		return nil, fmt.Errorf("unsorted string set")
	}
	for index := 1; index < len(values); index++ {
		if values[index-1] == values[index] {
			return nil, fmt.Errorf("duplicate string set value")
		}
	}
	return values, nil
}

func exactInteger(raw []byte, expected int) bool {
	return strings.TrimSpace(string(raw)) == fmt.Sprint(expected)
}

func validBoolean(raw []byte) bool {
	value := strings.TrimSpace(string(raw))
	return value == "true" || value == "false"
}

func stringEnum(raw []byte, allowed ...string) bool {
	value, ok := jsonString(raw)
	if !ok {
		return false
	}
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validJSONString(raw []byte, minimum, maximum int, valid func(string) bool) bool {
	value, ok := jsonString(raw)
	return ok && utf8.RuneCountInString(value) >= minimum && utf8.RuneCountInString(value) <= maximum && valid(value)
}

func jsonString(raw []byte) (string, bool) {
	var value string
	if jsonKind(raw) != '"' || json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}

func jsonKind(raw []byte) byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return 0
	}
	return trimmed[0]
}

func hasDuplicateKeyOrTrailingContent(value []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	if duplicateJSONValue(decoder) {
		return true
	}
	_, err := decoder.Token()
	return err != io.EOF
}

func duplicateJSONValue(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return true
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return false
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, keyOK := keyToken.(string)
			if err != nil || !keyOK {
				return true
			}
			if _, exists := seen[key]; exists {
				return true
			}
			seen[key] = struct{}{}
			if duplicateJSONValue(decoder) {
				return true
			}
		}
		closing, err := decoder.Token()
		return err != nil || closing != json.Delim('}')
	case '[':
		for decoder.More() {
			if duplicateJSONValue(decoder) {
				return true
			}
		}
		closing, err := decoder.Token()
		return err != nil || closing != json.Delim(']')
	default:
		return true
	}
}

func (requirement ArtifactRequirement) TenantID() shared.UUID { return requirement.tenantID }
func (requirement ArtifactRequirement) ArtifactVersionID() shared.UUID {
	return requirement.artifactVersionID
}
func (requirement ArtifactRequirement) RequirementDigest() shared.Digest {
	return requirement.requirementDigest
}
func (requirement ArtifactRequirement) NormalizedRequirement() []byte {
	return append([]byte(nil), requirement.normalizedRequirement...)
}
