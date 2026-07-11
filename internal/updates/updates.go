package updates

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"pbs-win-backup/internal/version"
)

// GitHubRepo is set at build time or left empty to skip remote check.
var GitHubRepo = ""

type Result struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	UpdateAvailable bool  `json:"update_available"`
	URL            string `json:"url,omitempty"`
	Message        string `json:"message"`
}

func Check() Result {
	cur := version.Version
	res := Result{
		CurrentVersion: cur,
		LatestVersion:  cur,
		Message:        "Проверка обновлений отключена (укажите GitHubRepo при сборке)",
	}
	if GitHubRepo == "" {
		return res
	}
	parts := strings.SplitN(GitHubRepo, "/", 2)
	if len(parts) != 2 {
		res.Message = "неверный формат GitHubRepo"
		return res
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", parts[0], parts[1])
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		res.Message = "не удалось проверить: " + err.Error()
		return res
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		res.Message = fmt.Sprintf("GitHub HTTP %d", resp.StatusCode)
		return res
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		res.Message = err.Error()
		return res
	}
	latest := strings.TrimPrefix(body.TagName, "v")
	res.LatestVersion = latest
	res.URL = body.HTMLURL
	res.UpdateAvailable = latest != "" && latest != cur
	if res.UpdateAvailable {
		res.Message = fmt.Sprintf("Доступна версия %s", latest)
	} else {
		res.Message = "Установлена актуальная версия"
	}
	return res
}
