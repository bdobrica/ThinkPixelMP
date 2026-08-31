package shared

import (
	"fmt"
	"strings"
)

// ArtifactReference is the tenant-relative logical {namespace}/{name} key.
type ArtifactReference struct {
	namespace string
	name      string
}

func ParseArtifactReference(value string) (ArtifactReference, error) {
	if len(value) < 3 || len(value) > 319 {
		return ArtifactReference{}, fmt.Errorf("artifact reference: invalid length")
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return ArtifactReference{}, fmt.Errorf("artifact reference: namespace and name required")
	}
	for _, part := range parts {
		if !validNameToken(part) {
			return ArtifactReference{}, fmt.Errorf("artifact reference: invalid name token")
		}
	}
	namespace := strings.Join(parts[:len(parts)-1], "/")
	if len(namespace) > 255 {
		return ArtifactReference{}, fmt.Errorf("artifact reference: namespace too long")
	}
	return ArtifactReference{namespace: namespace, name: parts[len(parts)-1]}, nil
}
func validNameToken(value string) bool {
	if value == "" || len(value) > 63 {
		return false
	}
	for index, character := range value {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if index > 0 && index < len(value)-1 {
			valid = valid || character == '-'
		}
		if !valid {
			return false
		}
	}
	return true
}
func (reference ArtifactReference) Namespace() string { return reference.namespace }
func (reference ArtifactReference) Name() string      { return reference.name }
func (reference ArtifactReference) String() string {
	if reference.namespace == "" || reference.name == "" {
		return ""
	}
	return reference.namespace + "/" + reference.name
}
