package webui

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/zedage/phorga/internal/config"
	"github.com/zedage/phorga/internal/synology"
)

// Defaults for entries added in the UI (WEB-015). New entries start as dry
// runs, so adding one never changes a file before the user checks the log.
const (
	newSourceMode       = "copy"
	newSourceSchedule   = "0 2 * * *"
	newDedupSchedule    = "0 4 * * *"
	newDedupMode        = "metadata"
	newDedupAction      = "skip"
	newTargetStructure  = "{year}/{month}"
	synologyUserField   = "synologyUser"
	formOpField         = "op"
	formOpSeparator     = ":"
	opAddUsers          = "addUsers"
	opAddProfile        = "addProfile"
	opRemoveProfile     = "removeProfile"
	opAddSource         = "addSource"
	opRemoveSource      = "removeSource"
	opAddDedup          = "addDedup"
	opRemoveDedup       = "removeDedup"
	opAddDedupSource    = "addDedupSource"
	opRemoveDedupSource = "removeDedupSource"
	opRefreshUsers      = "refreshUsers"
)

// synologyView is the Synology Photos section of the page (SYN-003).
type synologyView struct {
	Homes string // folder that was searched
	Found bool   // the homes folder could be read
	Users []synologyUserView
}

type synologyUserView struct {
	synology.User
	Configured bool
}

// Addable reports whether at least one listed user can still be added.
func (v synologyView) Addable() bool {
	for _, u := range v.Users {
		if u.Readable && !u.Configured {
			return true
		}
	}
	return false
}

// discoverUsers lists the Synology Photos users below the form's homes
// folder and marks the ones the form already has a profile for.
func discoverUsers(form configForm) synologyView {
	homes := strings.TrimSpace(form.SynologyHomes)
	if homes == "" {
		homes = config.DefaultSynologyHomes
	}
	view := synologyView{Homes: homes}
	users, err := synology.Discover(homes)
	if err != nil {
		return view
	}
	view.Found = true
	profiles := form.toConfig().Organizer
	for _, u := range users {
		view.Users = append(view.Users, synologyUserView{User: u, Configured: synology.Configured(profiles, u)})
	}
	return view
}

// handleFormOp applies one edit to the submitted form (add or remove an
// entry, add Synology Photos users) and shows the page again. It never
// writes a file and never changes the running configuration (WEB-015).
func (s *Server) handleFormOp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	lang := s.resolveLanguage(w, r)
	form, err := parseForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	focus := applyFormOp(&form, r.FormValue(formOpField), r.Form[synologyUserField])
	data := s.newPageData(lang, form)
	data.Focus = focus
	s.renderPage(w, data)
}

// applyFormOp changes the form as op asks and returns the id of the element
// to scroll to. Unknown ops and out-of-range indexes change nothing.
func applyFormOp(form *configForm, op string, users []string) string {
	parts := strings.Split(op, formOpSeparator)
	idx := func(n int) (int, bool) {
		if len(parts) <= n {
			return 0, false
		}
		v, err := strconv.Atoi(parts[n])
		return v, err == nil && v >= 0
	}
	i, okI := idx(1)
	j, okJ := idx(2)
	hasProfile := okI && i < len(form.Organizer)
	hasDedup := okI && i < len(form.DuplicateDetection)

	switch parts[0] {
	case opAddUsers:
		return addSynologyUsers(form, users)
	case opRefreshUsers:
		return "sec-synology"
	case opAddProfile:
		form.Organizer = append(form.Organizer, organizerProfileForm{
			DryRun:          true,
			TargetStructure: newTargetStructure,
			Sources:         []organizerSourceForm{newSourceForm()},
		})
		return "org-" + strconv.Itoa(len(form.Organizer)-1)
	case opRemoveProfile:
		if hasProfile {
			form.Organizer = append(form.Organizer[:i], form.Organizer[i+1:]...)
		}
		return "sec-organizer"
	case opAddSource:
		if hasProfile {
			p := &form.Organizer[i]
			p.Sources = append(p.Sources, newSourceForm())
			return "org-" + strconv.Itoa(i) + "-src-" + strconv.Itoa(len(p.Sources)-1)
		}
	case opRemoveSource:
		if hasProfile && okJ && j < len(form.Organizer[i].Sources) {
			p := &form.Organizer[i]
			p.Sources = append(p.Sources[:j], p.Sources[j+1:]...)
			return "org-" + strconv.Itoa(i)
		}
	case opAddDedup:
		form.DuplicateDetection = append(form.DuplicateDetection, duplicateDetectionForm{
			Enabled:       true,
			DryRun:        true,
			Schedule:      newDedupSchedule,
			Sources:       []string{""},
			DetectionMode: newDedupMode,
			ActionMethod:  newDedupAction,
		})
		return "dd-" + strconv.Itoa(len(form.DuplicateDetection)-1)
	case opRemoveDedup:
		if hasDedup {
			form.DuplicateDetection = append(form.DuplicateDetection[:i], form.DuplicateDetection[i+1:]...)
		}
		return "sec-dedup"
	case opAddDedupSource:
		if hasDedup {
			form.DuplicateDetection[i].Sources = append(form.DuplicateDetection[i].Sources, "")
			return "dd-" + strconv.Itoa(i)
		}
	case opRemoveDedupSource:
		if hasDedup && okJ && j < len(form.DuplicateDetection[i].Sources) {
			dd := &form.DuplicateDetection[i]
			dd.Sources = append(dd.Sources[:j], dd.Sources[j+1:]...)
			return "dd-" + strconv.Itoa(i)
		}
	}
	return ""
}

func newSourceForm() organizerSourceForm {
	return organizerSourceForm{Mode: newSourceMode, Schedule: newSourceSchedule}
}

// addSynologyUsers adds a profile (SYN-002) for each chosen user that is
// found in the homes folder and not configured yet. Names that discovery
// does not report are ignored, so the request cannot point at other folders.
func addSynologyUsers(form *configForm, names []string) string {
	view := discoverUsers(*form)
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}
	first := -1
	for _, u := range view.Users {
		if !wanted[u.Name] || u.Configured || !u.Readable {
			continue
		}
		form.Organizer = append(form.Organizer, newProfileForm(synology.Profile(u.User)))
		if first < 0 {
			first = len(form.Organizer) - 1
		}
	}
	if first < 0 {
		return "sec-synology"
	}
	return "org-" + strconv.Itoa(first)
}
