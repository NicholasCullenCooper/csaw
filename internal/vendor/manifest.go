package vendor

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/NicholasCullenCooper/csaw/internal/sources"
)

// ManifestFileName is the source-registry config file that declares vendors
// (alongside profiles). Same file csaw uses for profile config; vendors
// live under a top-level `vendors:` key.
const ManifestFileName = "csaw.yml"

// LoadDeclarations reads the `vendors:` block from the registry's csaw.yml.
// Returns nil + nil error if the file exists but has no vendors block, or
// if csaw.yml doesn't exist.
//
// Each declaration's URL is normalized through the shorthand parser
// (gh:/gl:/bb: prefixes resolved to canonical git URLs); the parser also
// extracts any `#ref` from shorthand and writes it to the Ref field if
// not explicitly set.
func LoadDeclarations(registryRoot string) ([]Declaration, error) {
	path := filepath.Join(registryRoot, ManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var raw struct {
		Vendors map[string]Declaration `yaml:"vendors"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse vendors block from %s: %w", path, err)
	}

	if len(raw.Vendors) == 0 {
		return nil, nil
	}

	out := make([]Declaration, 0, len(raw.Vendors))
	for name, d := range raw.Vendors {
		d.Name = name
		if d.URL == "" {
			return nil, fmt.Errorf("vendor %q in %s: missing required url", name, path)
		}
		// Normalize shorthand to canonical URL + extract ref if embedded.
		if sources.IsShorthand(d.URL) {
			parsed, err := sources.ParseShorthand(d.URL)
			if err != nil {
				return nil, fmt.Errorf("vendor %q in %s: %w", name, path, err)
			}
			d.URL = parsed.URL
			if d.Ref == "" {
				d.Ref = parsed.Ref
			}
		}
		out = append(out, d)
	}
	return out, nil
}

// AddDeclaration appends a vendor entry to csaw.yml at the registry root,
// creating the `vendors:` block if absent. Refuses to overwrite an
// existing vendor with the same name — caller should `csaw vendor remove`
// then `csaw vendor add` to replace.
//
// Preserves existing csaw.yml content via a two-step edit: parse → modify
// the vendors map → re-serialize. This loses comments in the file (a known
// limitation; the file is config, not narrative).
func AddDeclaration(registryRoot string, decl Declaration) error {
	if decl.Name == "" {
		return fmt.Errorf("vendor name is required")
	}
	if decl.URL == "" {
		return fmt.Errorf("vendor URL is required")
	}

	path := filepath.Join(registryRoot, ManifestFileName)
	var content map[string]interface{}

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, &content); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	case os.IsNotExist(err):
		content = map[string]interface{}{}
	default:
		return fmt.Errorf("read %s: %w", path, err)
	}
	if content == nil {
		content = map[string]interface{}{}
	}

	vendorsRaw, _ := content["vendors"]
	var vendors map[string]interface{}
	switch v := vendorsRaw.(type) {
	case map[string]interface{}:
		vendors = v
	case map[interface{}]interface{}:
		// Older yaml.v3 idiom; normalize.
		vendors = map[string]interface{}{}
		for k, val := range v {
			if ks, ok := k.(string); ok {
				vendors[ks] = val
			}
		}
	default:
		vendors = map[string]interface{}{}
	}

	if _, exists := vendors[decl.Name]; exists {
		return fmt.Errorf("vendor %q already declared in %s; remove it first to replace", decl.Name, path)
	}

	entry := map[string]interface{}{"url": decl.URL}
	if decl.Ref != "" {
		entry["ref"] = decl.Ref
	}
	if len(decl.Include) > 0 {
		entry["include"] = decl.Include
	}
	if len(decl.Exclude) > 0 {
		entry["exclude"] = decl.Exclude
	}
	vendors[decl.Name] = entry
	content["vendors"] = vendors

	out, err := yaml.Marshal(content)
	if err != nil {
		return fmt.Errorf("marshal updated %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// RemoveDeclaration deletes a vendor entry from csaw.yml. Returns an
// error if the vendor wasn't declared (silent removal would mask typos).
func RemoveDeclaration(registryRoot, name string) error {
	if name == "" {
		return fmt.Errorf("vendor name is required")
	}
	path := filepath.Join(registryRoot, ManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var content map[string]interface{}
	if err := yaml.Unmarshal(data, &content); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	vendorsRaw, ok := content["vendors"]
	if !ok {
		return fmt.Errorf("no vendors block in %s", path)
	}
	vendors, ok := vendorsRaw.(map[string]interface{})
	if !ok {
		if v, ok := vendorsRaw.(map[interface{}]interface{}); ok {
			vendors = map[string]interface{}{}
			for k, val := range v {
				if ks, ok := k.(string); ok {
					vendors[ks] = val
				}
			}
		} else {
			return fmt.Errorf("vendors block in %s has unexpected type", path)
		}
	}

	if _, exists := vendors[name]; !exists {
		return fmt.Errorf("vendor %q not declared in %s", name, path)
	}
	delete(vendors, name)

	if len(vendors) == 0 {
		delete(content, "vendors")
	} else {
		content["vendors"] = vendors
	}

	out, err := yaml.Marshal(content)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
