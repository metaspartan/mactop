package i18n

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

var (
	localeKeyRE      = regexp.MustCompile(`^([A-Za-z0-9_][A-Za-z0-9_]*)\s*=`)
	fmtPlaceholderRE = regexp.MustCompile(`%(\[[0-9]+\])?[-+# 0-9.*]*[bcdeEfFgGosxXvTtU%]`)
)

func TestLocalesMatchEnglishCatalog(t *testing.T) {
	files, err := localeFS.ReadDir("locales")
	if err != nil {
		t.Fatalf("read locales dir: %v", err)
	}

	englishName := "active.en.toml"
	englishCatalog, englishDeclared := loadLocaleCatalog(t, englishName)
	englishKeys := sortedKeys(englishCatalog)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".toml") || file.Name() == englishName {
			continue
		}

		catalog, declared := loadLocaleCatalog(t, file.Name())
		keys := sortedKeys(catalog)

		if missing := diffStrings(englishKeys, keys); len(missing) > 0 {
			t.Fatalf("%s is missing keys: %s", file.Name(), strings.Join(missing, ", "))
		}
		if extra := diffStrings(keys, englishKeys); len(extra) > 0 {
			t.Fatalf("%s has unexpected keys: %s", file.Name(), strings.Join(extra, ", "))
		}

		if dupes := duplicateKeys(declared); len(dupes) > 0 {
			t.Fatalf("%s declares duplicate keys: %s", file.Name(), strings.Join(dupes, ", "))
		}

		for _, key := range englishDeclared {
			want := placeholdersFor(englishCatalog[key])
			got := placeholdersFor(catalog[key])
			if !slices.Equal(want, got) {
				t.Fatalf("%s key %s has placeholder mismatch: want %v, got %v", file.Name(), key, want, got)
			}
		}
	}

	if dupes := duplicateKeys(englishDeclared); len(dupes) > 0 {
		t.Fatalf("%s declares duplicate keys: %s", englishName, strings.Join(dupes, ", "))
	}
}

func loadLocaleCatalog(t *testing.T, name string) (map[string]string, []string) {
	t.Helper()

	data, err := localeFS.ReadFile("locales/" + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	declared := declaredKeys(string(data))

	var catalog map[string]string
	if err := toml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}

	return catalog, declared
}

func declaredKeys(content string) []string {
	var keys []string
	for line := range strings.SplitSeq(content, "\n") {
		matches := localeKeyRE.FindStringSubmatch(line)
		if len(matches) == 2 {
			keys = append(keys, matches[1])
		}
	}
	return keys
}

func duplicateKeys(keys []string) []string {
	seen := make(map[string]bool, len(keys))
	var dupes []string
	for _, key := range keys {
		if seen[key] {
			dupes = append(dupes, key)
			continue
		}
		seen[key] = true
	}
	return dupes
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func diffStrings(a, b []string) []string {
	lookup := make(map[string]bool, len(b))
	for _, item := range b {
		lookup[item] = true
	}

	var diff []string
	for _, item := range a {
		if !lookup[item] {
			diff = append(diff, item)
		}
	}
	return diff
}

func placeholdersFor(msg string) []string {
	matches := fmtPlaceholderRE.FindAllString(msg, -1)
	placeholders := make([]string, 0, len(matches))
	for _, match := range matches {
		if match == "%%" {
			continue
		}
		placeholders = append(placeholders, match)
	}
	return placeholders
}
