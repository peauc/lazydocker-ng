package config

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/jesseduffield/yaml"
)

// unknownKeys returns the dotted paths of the keys in doc that no UserConfig
// field reads, in document order, e.g. "gui.scrollHieght" or
// "customCommands.containers[0].comand". Old spellings from legacyKeys and
// the keys of map fields (such as replacements.imageNamePrefixes) are known.
func unknownKeys(doc yaml.MapSlice) []string {
	var unknown []string
	walkMapping(doc, reflect.TypeFor[UserConfig](), "", nil, &unknown)
	return unknown
}

// walkMapping checks the keys of m against struct type t. path is the dotted
// display path of m; keys is the same path without list indices, used to match
// legacyKeys parents.
func walkMapping(m yaml.MapSlice, t reflect.Type, path string, keys []string, unknown *[]string) {
	fields := yamlFields(t)
	for _, item := range m {
		name := fmt.Sprint(item.Key)
		childPath := name
		if path != "" {
			childPath = path + "." + name
		}

		field, ok := fields[name]
		if !ok {
			if !isLegacyKey(keys, name) {
				*unknown = append(*unknown, childPath)
			}
			continue
		}
		walkValue(item.Value, field.Type, childPath, append(slices.Clip(keys), name), unknown)
	}
}

func walkValue(value any, t reflect.Type, path string, keys []string, unknown *[]string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		if m, ok := value.(yaml.MapSlice); ok {
			walkMapping(m, t, path, keys, unknown)
		}
	case reflect.Slice:
		if items, ok := value.([]any); ok {
			for i, item := range items {
				walkValue(item, t.Elem(), fmt.Sprintf("%s[%d]", path, i), keys, unknown)
			}
		}
	default:
		// maps accept any key; scalars have no keys
	}
}

// yamlFields maps each yaml key of struct type t to its field, using the same
// rule as the yaml decoder: the tag name, or the lowercased field name when the
// field has no tag.
func yamlFields(t reflect.Type) map[string]reflect.StructField {
	fields := map[string]reflect.StructField{}
	for field := range t.Fields() {
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		fields[name] = field
	}
	return fields
}

func isLegacyKey(parent []string, name string) bool {
	for _, key := range legacyKeys {
		if key.old == name && slices.Equal(key.parent, parent) {
			return true
		}
	}
	return false
}
