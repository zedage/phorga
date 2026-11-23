package webui

import (
	"bytes"
	"html/template"
	"net/http"
	"os"

	"github.com/zedage/phorga/internal/synology"
)

// ComposeSynology is the docker-compose.yml of the Synology guide. The
// README shows the same text (WEB-022).
const ComposeSynology = `services:
  phorga:
    image: mnrd001/phorga:latest
    container_name: phorga
    restart: unless-stopped
    user: "1030:100"                 # uid:gid of the DSM user "phorga"
    ports:
      - "127.0.0.1:8080:8080"        # NAS only; the reverse proxy opens it to your network
    volumes:
      - ./config:/config             # config.yaml, written by the web UI
      - ./state:/data/state
      - /volume1/homes:/homes        # the Synology Photos libraries of all users
    environment:
      - TZ=Europe/Berlin
      - PHORGA_WEB_UI_PASSWORD=choose-a-long-password
`

// ComposeOther is the docker-compose.yml for another computer that reaches
// the NAS over the network. The README shows the same text (WEB-022).
const ComposeOther = `services:
  phorga:
    image: mnrd001/phorga:latest
    container_name: phorga
    restart: unless-stopped
    user: "501:20"                   # output of "id -u" and "id -g" on this computer
    ports:
      - "127.0.0.1:8080:8080"        # this computer only; "8080:8080" opens it to your network
    volumes:
      - ./config:/config             # config.yaml, written by the web UI
      - ./state:/data/state
      - /Volumes/homes:/homes        # the NAS share "homes"; Linux: /mnt/nas-homes:/homes
    environment:
      - TZ=Europe/Berlin
      - PHORGA_WEB_UI_PASSWORD=choose-a-long-password
`

// helpData is shown on the help page (WEB-022).
type helpData struct {
	ConfigPath      string
	ConfigExists    bool
	Homes           string
	HomesFound      bool
	Users           int
	PasswordSet     bool
	Address         string
	ComposeSynology string
	ComposeOther    string
	// Guide is the guide in the chosen language, rendered from help_<lang>.
	Guide template.HTML
}

func (s *Server) helpData() helpData {
	cfg := s.currentConfig()
	data := helpData{
		ConfigPath:      s.configPath,
		Homes:           cfg.SynologyHomes(),
		PasswordSet:     s.passwordSet,
		Address:         s.address,
		ComposeSynology: ComposeSynology,
		ComposeOther:    ComposeOther,
	}
	if _, err := os.Stat(s.configPath); err == nil {
		data.ConfigExists = true
	}
	if users, err := synology.Discover(data.Homes); err == nil {
		data.HomesFound = true
		data.Users = len(users)
	}
	return data
}

// handleHelp shows the setup guide in the chosen language, English if
// there is none for it (WEB-022).
func (s *Server) handleHelp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lang := s.resolveLanguage(w, r)
	data := s.helpData()
	guide := "help_" + lang
	if s.templates.Lookup(guide) == nil {
		guide = "help_en"
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, guide, map[string]any{
		"Data": data,
		"Lang": lang,
		"T":    func(key string) string { return t(lang, key) },
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.Guide = template.HTML(buf.String())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.execute(w, "help", lang, data)
}
