package logging

import (
	"log/slog"
	"reflect"
	"strings"
)

const (
	maxDepth             = 8
	maxCollectionEntries = 64
	maxStringBytes       = 4 << 10
)

type classifiedValue struct{}

// Confidential marks C2 data for unconditional suppression.
func Confidential(any) any { return classifiedValue{} }

// Restricted marks C3 data for unconditional suppression.
func Restricted(any) any { return classifiedValue{} }

func sanitizeAttrs(attrs []slog.Attr, depth int) []slog.Attr {
	limit := min(len(attrs), maxCollectionEntries)
	clean := make([]slog.Attr, 0, limit+1)
	for _, attr := range attrs[:limit] {
		if sanitized, ok := sanitizeAttr(attr, depth); ok {
			clean = append(clean, sanitized)
		}
	}
	if len(attrs) > limit {
		clean = append(clean, slog.String("logging_truncated", TruncatedMarker))
	}
	return clean
}

func sanitizeAttr(attr slog.Attr, depth int) (slog.Attr, bool) {
	if attr.Key == "" || reservedCorrelationKey(attr.Key) {
		return slog.Attr{}, false
	}
	if sensitiveKey(attr.Key) {
		return slog.String(attr.Key, RedactedMarker), true
	}
	if depth >= maxDepth {
		return slog.String(attr.Key, TruncatedMarker), true
	}
	if _, isError := attr.Value.Any().(error); isError {
		return slog.String(attr.Key, RedactedMarker), true
	}
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		return slog.Attr{Key: attr.Key, Value: slog.GroupValue(sanitizeAttrs(value.Group(), depth+1)...)}, true
	}
	if value.Kind() == slog.KindAny {
		return slog.Any(attr.Key, sanitizeAny(value.Any(), depth+1, map[visit]bool{})), true
	}
	if value.Kind() == slog.KindString {
		return slog.String(attr.Key, boundedString(value.String())), true
	}
	return slog.Attr{Key: attr.Key, Value: value}, true
}

type visit struct {
	kind    reflect.Kind
	pointer uintptr
}

func sanitizeAny(input any, depth int, seen map[visit]bool) any {
	if input == nil {
		return nil
	}
	if _, ok := input.(classifiedValue); ok {
		return RedactedMarker
	}
	if _, ok := input.(error); ok {
		return RedactedMarker
	}
	if depth > maxDepth {
		return TruncatedMarker
	}
	return sanitizeReflect(reflect.ValueOf(input), depth, seen)
}

func sanitizeReflect(value reflect.Value, depth int, seen map[visit]bool) any {
	if !value.IsValid() {
		return nil
	}
	if depth > maxDepth {
		return TruncatedMarker
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		return sanitizeReflect(value.Elem(), depth, seen)
	}
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		key := visit{kind: value.Kind(), pointer: value.Pointer()}
		if seen[key] {
			return CycleMarker
		}
		seen[key] = true
		defer delete(seen, key)
		return sanitizeReflect(value.Elem(), depth+1, seen)
	case reflect.String:
		return boundedString(value.String())
	case reflect.Bool:
		return value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Interface()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Interface()
	case reflect.Float32, reflect.Float64:
		return value.Interface()
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return UnsupportedMarker
		}
		clean := make(map[string]any, min(value.Len(), maxCollectionEntries))
		iterator := value.MapRange()
		count := 0
		for iterator.Next() {
			if count >= maxCollectionEntries {
				clean["logging_truncated"] = TruncatedMarker
				break
			}
			key := iterator.Key().String()
			count++
			if reservedCorrelationKey(key) {
				continue
			}
			if sensitiveKey(key) {
				clean[key] = RedactedMarker
			} else {
				clean[key] = sanitizeReflect(iterator.Value(), depth+1, seen)
			}
		}
		return clean
	case reflect.Struct:
		clean := make(map[string]any)
		typ := value.Type()
		count := 0
		for index := 0; index < value.NumField(); index++ {
			field := typ.Field(index)
			current := value.Field(index)
			if !field.IsExported() || !current.CanInterface() {
				continue
			}
			name := field.Name
			if tag := strings.Split(field.Tag.Get("json"), ",")[0]; tag == "-" {
				continue
			} else if tag != "" {
				name = tag
			}
			if reservedCorrelationKey(name) {
				continue
			}
			if count >= maxCollectionEntries {
				clean["logging_truncated"] = TruncatedMarker
				break
			}
			count++
			if sensitiveKey(name) {
				clean[name] = RedactedMarker
			} else {
				clean[name] = sanitizeReflect(current, depth+1, seen)
			}
		}
		return clean
	case reflect.Slice, reflect.Array:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return nil
		}
		limit := min(value.Len(), maxCollectionEntries)
		clean := make([]any, 0, limit+1)
		for index := 0; index < limit; index++ {
			clean = append(clean, sanitizeReflect(value.Index(index), depth+1, seen))
		}
		if value.Len() > limit {
			clean = append(clean, TruncatedMarker)
		}
		return clean
	default:
		return UnsupportedMarker
	}
}

func boundedString(value string) string {
	if len(value) <= maxStringBytes {
		return value
	}
	cut := maxStringBytes - len(TruncatedMarker)
	for cut > 0 && value[cut]&0xc0 == 0x80 {
		cut--
	}
	return value[:cut] + TruncatedMarker
}

func reservedCorrelationKey(key string) bool {
	_, ok := map[string]struct{}{"event": {}, "time": {}, "level": {}, "msg": {}, "source": {}, "tenant": {}, "publisher_id": {}, "artifact_id": {}, "artifact_version_id": {}, "artifact_digest": {}, "catalog_id": {}, "promotion_request_id": {}, "resolution_id": {}, "import_source_id": {}, "request_id": {}, "trace_id": {}}[normalizeKey(key)]
	return ok
}

func sensitiveKey(key string) bool {
	normalized := normalizeKey(key)
	exact := map[string]struct{}{"authorization": {}, "proxy_authorization": {}, "cookie": {}, "set_cookie": {}, "password": {}, "passwd": {}, "secret": {}, "credential": {}, "credentials": {}, "token": {}, "access_token": {}, "refresh_token": {}, "id_token": {}, "api_key": {}, "client_secret": {}, "private_key": {}, "signing_key": {}, "signing_material": {}, "database_url": {}, "database_dsn": {}, "dsn": {}, "request_body": {}, "response_body": {}, "descriptor": {}, "evidence": {}, "evidence_payload": {}, "policy_input": {}, "prompt": {}, "source_content": {}, "raw_content": {}, "query": {}, "query_string": {}}
	if _, ok := exact[normalized]; ok {
		return true
	}
	for _, fragment := range []string{"authorization", "password", "passwd", "secret", "token", "api_key", "cookie", "credential", "private_key", "signing_key", "descriptor", "evidence", "policy_input", "prompt", "source_content"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	for _, suffix := range []string{"_password", "_passwd", "_secret", "_token", "_api_key", "_credential", "_credentials", "_private_key", "_dsn", "_body", "_payload"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}

func normalizeKey(key string) string {
	return strings.ToLower(strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(strings.TrimSpace(key)))
}
