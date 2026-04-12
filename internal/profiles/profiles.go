package profiles

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/NicholasCullenCooper/csaw/internal/runtime"
)

type Profile struct {
	Name           string
	Description    string
	Include        []string
	Exclude        []string
	IncludeIgnored bool
}

type Resolver interface {
	Resolve(name string) (Profile, error)
	All() (map[string]Profile, error)
}

type FileResolver struct {
	file        string
	definitions map[string]definition
}

type definition struct {
	Description    string
	Extends        []string
	Include        []string
	Exclude        []string
	IncludeIgnored bool
}

func NewFileResolver(file string) (*FileResolver, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal([]byte(runtime.StripBOM(string(content))), &raw); err != nil {
		return nil, err
	}

	definitions := make(map[string]definition, len(raw))
	for name, value := range raw {
		definition, err := normalizeDefinition(name, value)
		if err != nil {
			return nil, err
		}
		definitions[name] = definition
	}

	return &FileResolver{file: file, definitions: definitions}, nil
}

func (r *FileResolver) Resolve(name string) (Profile, error) {
	resolved, _, err := r.resolve(name, map[string]bool{})
	return resolved, err
}

func (r *FileResolver) All() (map[string]Profile, error) {
	result := make(map[string]Profile, len(r.definitions))
	for name := range r.definitions {
		profile, err := r.Resolve(name)
		if err != nil {
			return nil, err
		}
		result[name] = profile
	}
	return result, nil
}

func (r *FileResolver) resolve(name string, stack map[string]bool) (Profile, map[string]Profile, error) {
	if stack[name] {
		return Profile{}, nil, fmt.Errorf("profile cycle detected while resolving %q", name)
	}

	definition, ok := r.definitions[name]
	if !ok {
		return Profile{}, nil, fmt.Errorf("unknown profile %q", name)
	}

	stack[name] = true
	defer delete(stack, name)

	profile := Profile{
		Name:        name,
		Description: definition.Description,
	}

	for _, parentName := range definition.Extends {
		parent, _, err := r.resolve(parentName, stack)
		if err != nil {
			return Profile{}, nil, err
		}
		profile.Include = mergeUnique(profile.Include, parent.Include)
		profile.Exclude = mergeUnique(profile.Exclude, parent.Exclude)
		profile.IncludeIgnored = profile.IncludeIgnored || parent.IncludeIgnored
	}

	profile.Include = mergeUnique(profile.Include, definition.Include)
	profile.Exclude = mergeUnique(profile.Exclude, definition.Exclude)
	profile.IncludeIgnored = profile.IncludeIgnored || definition.IncludeIgnored

	return profile, nil, nil
}

func normalizeDefinition(name string, value any) (definition, error) {
	switch typed := value.(type) {
	case []any:
		include, err := toStringSlice(typed)
		if err != nil {
			return definition{}, fmt.Errorf("profile %q: %w", name, err)
		}
		return definition{Include: include}, nil
	case map[string]any:
		return normalizeMapDefinition(name, typed)
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for key, item := range typed {
			stringKey, ok := key.(string)
			if !ok {
				return definition{}, fmt.Errorf("profile %q has a non-string field name", name)
			}
			converted[stringKey] = item
		}
		return normalizeMapDefinition(name, converted)
	default:
		return definition{}, fmt.Errorf("profile %q must be a list or object", name)
	}
}

func normalizeMapDefinition(name string, value map[string]any) (definition, error) {
	var result definition

	if description, ok := value["description"]; ok {
		text, ok := description.(string)
		if !ok {
			return definition{}, fmt.Errorf("profile %q has an invalid description", name)
		}
		result.Description = text
	}

	if includeIgnored, ok := value["includeIgnored"]; ok {
		flag, ok := includeIgnored.(bool)
		if !ok {
			return definition{}, fmt.Errorf("profile %q has an invalid includeIgnored", name)
		}
		result.IncludeIgnored = flag
	}

	if rawExtends, ok := value["extends"]; ok {
		extends, err := normalizeExtends(rawExtends)
		if err != nil {
			return definition{}, fmt.Errorf("profile %q: %w", name, err)
		}
		result.Extends = extends
	}

	if rawInclude, ok := value["include"]; ok {
		include, err := normalizeStringSlice(rawInclude)
		if err != nil {
			return definition{}, fmt.Errorf("profile %q: %w", name, err)
		}
		result.Include = include
	}

	if rawExclude, ok := value["exclude"]; ok {
		exclude, err := normalizeStringSlice(rawExclude)
		if err != nil {
			return definition{}, fmt.Errorf("profile %q: %w", name, err)
		}
		result.Exclude = exclude
	}

	if len(result.Include) == 0 && len(result.Extends) == 0 {
		return definition{}, fmt.Errorf("profile %q must define include or extends", name)
	}

	return result, nil
}

func normalizeExtends(value any) ([]string, error) {
	switch typed := value.(type) {
	case string:
		return []string{typed}, nil
	default:
		return normalizeStringSlice(value)
	}
}

func normalizeStringSlice(value any) ([]string, error) {
	switch typed := value.(type) {
	case []any:
		return toStringSlice(typed)
	case []string:
		return typed, nil
	default:
		return nil, errors.New("expected a string or list of strings")
	}
}

func toStringSlice(values []any) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("expected a list of strings")
		}
		result = append(result, text)
	}
	return result, nil
}

func mergeUnique(current []string, additions []string) []string {
	seen := make(map[string]struct{}, len(current))
	for _, item := range current {
		seen[item] = struct{}{}
	}

	for _, item := range additions {
		if _, ok := seen[item]; ok {
			continue
		}
		current = append(current, item)
		seen[item] = struct{}{}
	}

	return current
}

func SortedNames(values map[string]Profile) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func JoinPatterns(values []string) string {
	return strings.Join(values, ", ")
}
