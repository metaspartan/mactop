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

func TestNormalizeLanguageTag(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"english primary", "en", "en"},
		{"english region", "en-US", "en"},
		{"english POSIX", "en_US.UTF-8", "en"},
		{"japanese region", "ja-JP", "ja"},
		{"japanese POSIX", "ja_JP.UTF-8", "ja"},
		{"zh primary", "zh", "zh"},
		{"zh-CN", "zh-CN", "zh"},
		{"zh-Hans", "zh-Hans", "zh"},
		{"zh-Hans-CN", "zh-Hans-CN", "zh"},
		{"zh-SG", "zh-SG", "zh"},
		{"zh POSIX CN", "zh_CN.UTF-8", "zh"},
		{"zh-TW legacy", "zh-TW", "zh-Hant"},
		{"zh-HK legacy", "zh-HK", "zh-Hant"},
		{"zh-MO legacy", "zh-MO", "zh-Hant"},
		{"zh-Hant", "zh-Hant", "zh-Hant"},
		{"zh-Hant-TW", "zh-Hant-TW", "zh-Hant"},
		{"zh-Hant-HK", "zh-Hant-HK", "zh-Hant"},
		{"zh POSIX TW", "zh_TW.UTF-8", "zh-Hant"},
		{"zh POSIX HK", "zh_HK.UTF-8", "zh-Hant"},
		{"mixed case Hant", "ZH-hAnT-tw", "zh-Hant"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeLanguageTag(tc.in); got != tc.want {
				t.Fatalf("normalizeLanguageTag(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

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

func TestInitLoadsOnlyRequestedCatalogs(t *testing.T) {
	cases := []struct {
		lang string
		want []string
	}{
		{"en", []string{"en"}},
		{"ru", []string{"en", "ru"}},
		{"ru_RU.UTF-8", []string{"en", "ru"}},
		{"zh-TW", []string{"en", "zh", "zh-Hant"}},
		{"xx", []string{"en"}},
	}
	for _, tc := range cases {
		Init(tc.lang)
		var got []string
		for _, tag := range bundle.LanguageTags() {
			got = append(got, tag.String())
		}
		slices.Sort(got)
		if !slices.Equal(got, tc.want) {
			t.Errorf("Init(%q) loaded %v, want %v", tc.lang, got, tc.want)
		}
	}
}

func TestInitStillTranslates(t *testing.T) {
	const id = "Headless_ErrorInitMetrics"
	Init("en")
	en := T(id)
	for _, lang := range []string{"ru", "zh-TW", "ja-JP"} {
		Init(lang)
		if got := T(id); got == en || got == id {
			t.Errorf("Init(%q): T(%q) = %q, want a translation different from en %q", lang, id, got, en)
		}
	}
	Init("xx")
	if got := T(id); got != en {
		t.Errorf("unknown language should fall back to en: got %q, want %q", got, en)
	}
}
