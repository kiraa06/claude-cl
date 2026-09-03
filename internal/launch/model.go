package launch

import (
	"encoding/json"
	"os"
	"strings"
)

// Families are the model aliases the footer cycles through. Aliases rather than
// pinned ids, so the picker keeps working as new models ship.
var Families = []string{"opus", "sonnet", "haiku", "fable"}

// Models returns the footer's model list along with the index to start on.
// The model configured in settings comes first, so the default the user already
// chose is the one preselected.
func Models(settingsPath string) ([]string, int) {
	list := make([]string, 0, len(Families)+1)
	if d := configuredModel(settingsPath); d != "" {
		list = append(list, d)
	}
	for _, f := range Families {
		if !contains(list, f) {
			list = append(list, f)
		}
	}
	return list, 0
}

// configuredModel reads the "model" setting, ignoring an unreadable or
// malformed settings file: a bad config should not stop the picker opening.
func configuredModel(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var settings struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(data, &settings) != nil {
		return ""
	}
	return settings.Model
}

// Alias maps a recorded model id such as "claude-sonnet-5" to the alias that
// selects its current generation. Unrecognised ids return "", leaving the
// user's configured default in charge rather than guessing.
func Alias(modelID string) string {
	if modelID == "" {
		return ""
	}
	if strings.HasPrefix(modelID, "grok") {
		return modelID
	}
	id := strings.TrimPrefix(modelID, "claude-")
	for _, f := range Families {
		if strings.HasPrefix(id, f) {
			return f
		}
	}
	return ""
}

// IndexOf finds a model in the footer list, or -1.
func IndexOf(models []string, model string) int {
	for i, m := range models {
		if m == model {
			return i
		}
	}
	return -1
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
