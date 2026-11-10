package config

import "github.com/jesseduffield/yaml"

// legacyKey is a config key whose spelling changed. The old spelling, used by
// upstream lazydocker and older lazydocker-ng configs, keeps working; when
// both spellings are present the current one wins.
type legacyKey struct {
	parent  []string              // mapping that holds the key; nil = top level
	old     string                // spelling used by upstream lazydocker and older lazydocker-ng
	current string                // canonical spelling (the struct's yaml tag)
	field   func(*UserConfig) any // pointer the old value decodes into
}

var legacyKeys = []legacyKey{
	{nil, "oS", "os", func(c *UserConfig) any { return &c.OS }},
	{[]string{"gui"}, "mouseEvents", "ignoreMouseEvents", func(c *UserConfig) any { return &c.Gui.IgnoreMouseEvents }},
	{[]string{"commandTemplates"}, "viewAlLogs", "viewAllLogs", func(c *UserConfig) any { return &c.CommandTemplates.ViewAllLogs }},
	{[]string{"commandTemplates"}, "preferedExecShell", "preferredExecShell", func(c *UserConfig) any { return &c.CommandTemplates.PreferredExecShells }},
}

// applyLegacyKeys decodes every old-spelling key of doc whose current spelling
// is absent into cfg, merging onto what is already there like the main decode.
func applyLegacyKeys(doc yaml.MapSlice, cfg *UserConfig) error {
	for _, key := range legacyKeys {
		parent, ok := mappingAt(doc, key.parent)
		if !ok {
			continue
		}
		value, hasOld := lookupKey(parent, key.old)
		if !hasOld {
			continue
		}
		if _, hasCurrent := lookupKey(parent, key.current); hasCurrent {
			continue
		}

		raw, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(raw, key.field(cfg)); err != nil {
			return err
		}
	}
	return nil
}

// mappingAt returns the mapping reached by following path from doc.
func mappingAt(doc yaml.MapSlice, path []string) (yaml.MapSlice, bool) {
	current := doc
	for _, name := range path {
		value, ok := lookupKey(current, name)
		if !ok {
			return nil, false
		}
		next, ok := value.(yaml.MapSlice)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func lookupKey(m yaml.MapSlice, name string) (any, bool) {
	for _, item := range m {
		if key, ok := item.Key.(string); ok && key == name {
			return item.Value, true
		}
	}
	return nil, false
}
