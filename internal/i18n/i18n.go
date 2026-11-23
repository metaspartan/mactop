package i18n

import (
	"embed"
	"os"
	"os/exec"
	"strings"

	"github.com/BurntSushi/toml"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*
var localeFS embed.FS

var (
	bundle    *goi18n.Bundle
	localizer *goi18n.Localizer
)

// Init initializes the i18n engine with the specified overriding language.
// If langOverride is empty, it attempts to detect the system language.
func Init(langOverride string) {
	bundle = goi18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	var langs []string

	if langOverride != "" {
		// Normalize explicit overrides (--lang, MACTOP_LANG, config.json) the
		// same way auto-detection does, so locales like zh-TW / zh-Hant-TW /
		// zh_TW.UTF-8 resolve to the shipped zh-Hant catalog and en-US -> en.
		// The normalized tag goes first; the raw override is kept as a
		// secondary preference in case a future catalog ships a more specific
		// tag than the normalized primary subtag.
		if tag := normalizeLanguageTag(langOverride); tag != "" && tag != langOverride {
			langs = append(langs, tag)
		}
		langs = append(langs, langOverride)
	} else {
		sysLang := detectSystemLanguage()
		if sysLang != "" {
			langs = append(langs, sysLang)
		}
	}

	// Always fallback to en
	langs = append(langs, "en")

	loadCatalogs(langs)

	localizer = goi18n.NewLocalizer(bundle, langs...)
}

// loadCatalogs parses only the embedded catalogs whose primary subtag matches
// one of the requested languages. Parsing all of them costs ~20 MB of
// allocations at startup and keeps the unused message templates alive for the
// whole run (~10 MB of footprint in a long-running --headless/-p process),
// while only the requested language and the en fallback can ever be selected.
func loadCatalogs(langs []string) {
	wanted := make(map[string]bool, len(langs))
	for _, l := range langs {
		wanted[primarySubtag(l)] = true
	}
	files, err := localeFS.ReadDir("locales")
	if err != nil {
		return
	}
	for _, f := range files {
		name := f.Name()
		if f.IsDir() || !strings.HasSuffix(name, ".toml") {
			continue
		}
		tag := strings.TrimSuffix(strings.TrimPrefix(name, "active."), ".toml")
		if !wanted[primarySubtag(tag)] {
			continue
		}
		_, _ = bundle.LoadMessageFileFS(localeFS, "locales/"+name)
	}
}

// primarySubtag returns the lower-cased primary language subtag of a BCP-47
// or POSIX locale ("zh-Hant" -> "zh", "pt_BR.UTF-8" -> "pt").
func primarySubtag(tag string) string {
	tag = strings.ToLower(strings.ReplaceAll(tag, "_", "-"))
	if i := strings.IndexAny(tag, "-."); i >= 0 {
		tag = tag[:i]
	}
	return tag
}

// T returns the localized string for a given Message ID.
// Returns the ID if translation is not found.
func T(id string) string {
	if localizer == nil {
		Init("")
	}
	msg, err := localizer.Localize(&goi18n.LocalizeConfig{
		MessageID: id,
	})
	if err != nil {
		return id
	}
	return msg
}

// TData returns the localized string for a given Message ID with template data.
func TData(id string, templateData map[string]any) string {
	if localizer == nil {
		Init("")
	}
	msg, err := localizer.Localize(&goi18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: templateData,
	})
	if err != nil {
		return id
	}
	return msg
}

// detectSystemLanguage attempts to read the macOS system language.
func detectSystemLanguage() string {
	if out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
		for line := range strings.SplitSeq(string(out), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "\"") {
				continue
			}
			if tag := normalizeLanguageTag(strings.Trim(line, "\",")); tag != "" {
				return tag
			}
		}
	}

	for _, env := range []string{"LC_ALL", "LANG"} {
		if tag := normalizeLanguageTag(os.Getenv(env)); tag != "" {
			return tag
		}
	}

	return ""
}

// normalizeLanguageTag maps a raw BCP-47 or POSIX locale to a shipped tag.
// zh-TW, zh-HK, zh-MO, and zh-Hant* return "zh-Hant"; other zh variants return
// "zh"; otherwise the primary subtag is kept.
func normalizeLanguageTag(raw string) string {
	if raw == "" {
		return ""
	}
	normalized := strings.ReplaceAll(raw, "_", "-")
	if dot := strings.Index(normalized, "."); dot != -1 {
		normalized = normalized[:dot]
	}
	parts := strings.Split(strings.ToLower(normalized), "-")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	if parts[0] == "zh" {
		for _, part := range parts[1:] {
			switch part {
			case "hant", "tw", "hk", "mo":
				return "zh-Hant"
			}
		}
		return "zh"
	}
	return parts[0]
}
