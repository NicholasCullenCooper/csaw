package profiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
	"github.com/NicholasCullenCooper/csaw/internal/sources"
)

type CatalogResolver struct {
	definitions map[string]definition
}

func NewCatalogResolver(paths runtime.Paths, catalog []sources.CatalogSource) (*CatalogResolver, error) {
	definitions := map[string]definition{}

	if err := loadDefinitionsInto(definitions, filepath.Join(paths.Root, runtime.ProfilesFile), ""); err != nil {
		return nil, err
	}
	for _, source := range catalog {
		if err := loadDefinitionsInto(definitions, filepath.Join(source.Root, runtime.ProfilesFile), source.Name); err != nil {
			return nil, err
		}
	}

	return &CatalogResolver{definitions: definitions}, nil
}

func (r *CatalogResolver) Resolve(name string) (Profile, error) {
	resolved, err := resolveDefinition(name, r.definitions, map[string]bool{})
	if err == nil {
		return resolved, nil
	}

	if !strings.Contains(name, "/") {
		var matches []string
		suffix := "/" + name
		for candidate := range r.definitions {
			if strings.HasSuffix(candidate, suffix) {
				matches = append(matches, candidate)
			}
		}
		if len(matches) == 1 {
			return resolveDefinition(matches[0], r.definitions, map[string]bool{})
		}
	}

	return Profile{}, err
}

func (r *CatalogResolver) All() (map[string]Profile, error) {
	results := make(map[string]Profile, len(r.definitions))
	for name := range r.definitions {
		profile, err := r.Resolve(name)
		if err != nil {
			return nil, err
		}
		results[name] = profile
	}
	return results, nil
}

func loadDefinitionsInto(target map[string]definition, file string, namespace string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(runtime.StripBOM(string(content))), &raw); err != nil {
		return err
	}

	for name, value := range raw {
		definition, err := normalizeDefinition(name, value)
		if err != nil {
			return err
		}

		key := qualifyProfileName(namespace, name)
		target[key] = qualifyDefinition(namespace, definition)
	}

	return nil
}

func qualifyProfileName(namespace string, name string) string {
	if namespace == "" || strings.Contains(name, "/") {
		return name
	}
	return namespace + "/" + name
}

func qualifyDefinition(namespace string, definition definition) definition {
	if namespace == "" {
		return definition
	}

	qualified := definition
	for index, parent := range definition.Extends {
		qualified.Extends[index] = qualifyProfileName(namespace, parent)
	}
	for index, pattern := range definition.Include {
		qualified.Include[index] = namespace + "/" + runtime.NormalizeRegistryPath(pattern)
	}
	for index, pattern := range definition.Exclude {
		qualified.Exclude[index] = namespace + "/" + runtime.NormalizeRegistryPath(pattern)
	}

	return qualified
}

func resolveDefinition(name string, definitions map[string]definition, stack map[string]bool) (Profile, error) {
	if stack[name] {
		return Profile{}, errors.New("profile cycle detected")
	}

	definition, ok := definitions[name]
	if !ok {
		return Profile{}, errors.New("unknown profile " + name)
	}

	stack[name] = true
	defer delete(stack, name)

	profile := Profile{
		Name:        name,
		Description: definition.Description,
	}

	for _, parentName := range definition.Extends {
		parent, err := resolveDefinition(parentName, definitions, stack)
		if err != nil {
			return Profile{}, err
		}
		profile.Include = mergeUnique(profile.Include, parent.Include)
		profile.Exclude = mergeUnique(profile.Exclude, parent.Exclude)
		profile.IncludeIgnored = profile.IncludeIgnored || parent.IncludeIgnored
	}

	profile.Include = mergeUnique(profile.Include, definition.Include)
	profile.Exclude = mergeUnique(profile.Exclude, definition.Exclude)
	profile.IncludeIgnored = profile.IncludeIgnored || definition.IncludeIgnored

	return profile, nil
}
