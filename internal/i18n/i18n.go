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

	// Load all translation files from the embedded FS
	files, err := localeFS.ReadDir("locales")
	if err == nil {
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".toml") {
				_, _ = bundle.LoadMessageFileFS(localeFS, "locales/"+f.Name())
			}
		}
	}

	var langs []string

	if langOverride != "" {
		langs = append(langs, langOverride)
	} else {
		sysLang := detectSystemLanguage()
		if sysLang != "" {
			langs = append(langs, sysLang)
		}
	}

	// Always fallback to en
	langs = append(langs, "en")

	localizer = goi18n.NewLocalizer(bundle, langs...)
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
	// First try macOS defaults command
	cmd := exec.Command("defaults", "read", "-g", "AppleLanguages")
	out, err := cmd.Output()
	if err == nil {
		// Output looks like:
		// (
		//     "en-US",
		//     "es-US"
		// )
		lines := strings.SplitSeq(string(out), "\n")
		for line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "\"") && strings.HasSuffix(line, "\",") {
				lang := strings.Trim(line, "\",")
				// Convert en-US to en, es-US to es
				parts := strings.Split(lang, "-")
				if len(parts) > 0 {
					return parts[0]
				}
				return lang
			} else if strings.HasPrefix(line, "\"") && strings.HasSuffix(line, "\"") {
				lang := strings.Trim(line, "\"")
				parts := strings.Split(lang, "-")
				if len(parts) > 0 {
					return parts[0]
				}
				return lang
			}
		}
	}

	// Fallback to standard environment variables
	if lang := os.Getenv("LC_ALL"); lang != "" {
		parts := strings.Split(lang, "_")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	if lang := os.Getenv("LANG"); lang != "" {
		parts := strings.Split(lang, "_")
		if len(parts) > 0 {
			return parts[0]
		}
	}

	return ""
}
