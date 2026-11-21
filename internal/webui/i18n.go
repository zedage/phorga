package webui

import "strings"

const defaultLanguage = "en"

var supportedLanguages = map[string]map[string]string{
	"de": {
		"page_title":           "Phorga Konfiguration",
		"heading":              "Phorga Web-Konfiguration",
		"intro":                "Zeigt die wirksame Konfiguration. Speichern in die aktive Datei wendet sie sofort an; Speichern in eine separate Datei schreibt nur diese Datei.",
		"loaded_config":        "Geladene Konfigurationsdatei",
		"effective_values":     "Aktuell geladene Werte",
		"state_dir":            "State-Verzeichnis",
		"webui_section":        "Web-UI",
		"webui_enabled":        "Web-UI aktiviert",
		"webui_address":        "Bind-Adresse",
		"organizer_section":    "Organizer-Profile",
		"organizer_profile":    "Organizer-Profil",
		"dry_run":              "Dry-Run",
		"target":               "Zielordner",
		"target_structure":     "Zielstruktur",
		"fallback_folder":      "Fallback-Ordner",
		"ignore_mod_time":      "Änderungsdatum nicht verwenden",
		"ntfy_url":             "ntfy URL",
		"sources":              "Quellen",
		"source":               "Quelle",
		"path":                 "Pfad",
		"mode":                 "Modus",
		"schedule":             "Zeitplan",
		"duplicates_enabled":   "Duplikatprüfung aktiv",
		"detection_mode":       "Erkennungsmodus",
		"action_method":        "Aktion",
		"duplicate_folder":     "Duplikate-Ordner",
		"duplicate_section":    "Dedup-Jobs",
		"duplicate_job":        "Dedup-Job",
		"enabled":              "Aktiviert",
		"save_target":          "Speicherziel",
		"save_target_active":   "In aktive Konfigurationsdatei speichern und anwenden",
		"save_target_separate": "In separate Datei speichern",
		"separate_path":        "Separater Dateipfad",
		"validate":             "Validieren",
		"save":                 "Speichern",
		"language":             "Sprache",
		"success_prefix":       "Erfolgreich gespeichert in",
		"validation_failed":    "Validierung fehlgeschlagen",
		"save_failed":          "Speichern fehlgeschlagen",
		"private_notice":       "Für lokale/private Nutzung vorgesehen. Nicht ohne zusätzlichen Schutz öffentlich freigeben.",
	},
	"en": {
		"page_title":           "Phorga Configuration",
		"heading":              "Phorga Web Configuration",
		"intro":                "Shows the configuration in effect. Saving to the active file applies it right away; saving to a separate file only writes that file.",
		"loaded_config":        "Loaded config file",
		"effective_values":     "Currently loaded values",
		"state_dir":            "State directory",
		"webui_section":        "Web UI",
		"webui_enabled":        "Web UI enabled",
		"webui_address":        "Bind address",
		"organizer_section":    "Organizer profiles",
		"organizer_profile":    "Organizer profile",
		"dry_run":              "Dry run",
		"target":               "Target folder",
		"target_structure":     "Target structure",
		"fallback_folder":      "Fallback folder",
		"ignore_mod_time":      "Ignore modification time",
		"ntfy_url":             "ntfy URL",
		"sources":              "Sources",
		"source":               "Source",
		"path":                 "Path",
		"mode":                 "Mode",
		"schedule":             "Schedule",
		"duplicates_enabled":   "Duplicate handling enabled",
		"detection_mode":       "Detection mode",
		"action_method":        "Action",
		"duplicate_folder":     "Duplicate folder",
		"duplicate_section":    "Duplicate detection jobs",
		"duplicate_job":        "Duplicate job",
		"enabled":              "Enabled",
		"save_target":          "Save target",
		"save_target_active":   "Save to the active config file and apply",
		"save_target_separate": "Save to separate file",
		"separate_path":        "Separate file path",
		"validate":             "Validate",
		"save":                 "Save",
		"language":             "Language",
		"success_prefix":       "Saved successfully to",
		"validation_failed":    "Validation failed",
		"save_failed":          "Saving failed",
		"private_notice":       "Intended for local/private use. Do not expose publicly without additional protection.",
	},
	"it": {
		"page_title":           "Configurazione Phorga",
		"heading":              "Configurazione Web di Phorga",
		"intro":                "Mostra la configurazione in vigore. Salvando nel file attivo la si applica subito; salvando in un file separato si scrive solo quel file.",
		"loaded_config":        "File di configurazione caricato",
		"effective_values":     "Valori attualmente caricati",
		"state_dir":            "Directory di stato",
		"webui_section":        "Web UI",
		"webui_enabled":        "Web UI abilitata",
		"webui_address":        "Indirizzo di bind",
		"organizer_section":    "Profili organizer",
		"organizer_profile":    "Profilo organizer",
		"dry_run":              "Dry run",
		"target":               "Cartella di destinazione",
		"target_structure":     "Struttura di destinazione",
		"fallback_folder":      "Cartella fallback",
		"ignore_mod_time":      "Ignora data di modifica",
		"ntfy_url":             "URL ntfy",
		"sources":              "Sorgenti",
		"source":               "Sorgente",
		"path":                 "Percorso",
		"mode":                 "Modalità",
		"schedule":             "Pianificazione",
		"duplicates_enabled":   "Gestione duplicati abilitata",
		"detection_mode":       "Modalità di rilevamento",
		"action_method":        "Azione",
		"duplicate_folder":     "Cartella duplicati",
		"duplicate_section":    "Job rilevamento duplicati",
		"duplicate_job":        "Job duplicati",
		"enabled":              "Abilitato",
		"save_target":          "Destinazione salvataggio",
		"save_target_active":   "Salva nel file di configurazione attivo e applica",
		"save_target_separate": "Salva in un file separato",
		"separate_path":        "Percorso file separato",
		"validate":             "Valida",
		"save":                 "Salva",
		"language":             "Lingua",
		"success_prefix":       "Salvato con successo in",
		"validation_failed":    "Validazione non riuscita",
		"save_failed":          "Salvataggio non riuscito",
		"private_notice":       "Pensato per uso locale/privato. Non esporre pubblicamente senza protezioni aggiuntive.",
	},
	"fr": {
		"page_title":           "Configuration Phorga",
		"heading":              "Configuration Web Phorga",
		"intro":                "Affiche la configuration en vigueur. Enregistrer dans le fichier actif l'applique tout de suite ; enregistrer dans un fichier séparé écrit seulement ce fichier.",
		"loaded_config":        "Fichier de configuration chargé",
		"effective_values":     "Valeurs actuellement chargées",
		"state_dir":            "Répertoire d'état",
		"webui_section":        "Interface Web",
		"webui_enabled":        "Interface Web activée",
		"webui_address":        "Adresse d'écoute",
		"organizer_section":    "Profils organizer",
		"organizer_profile":    "Profil organizer",
		"dry_run":              "Exécution à blanc",
		"target":               "Dossier cible",
		"target_structure":     "Structure cible",
		"fallback_folder":      "Dossier de repli",
		"ignore_mod_time":      "Ignorer la date de modification",
		"ntfy_url":             "URL ntfy",
		"sources":              "Sources",
		"source":               "Source",
		"path":                 "Chemin",
		"mode":                 "Mode",
		"schedule":             "Planification",
		"duplicates_enabled":   "Gestion des doublons activée",
		"detection_mode":       "Mode de détection",
		"action_method":        "Action",
		"duplicate_folder":     "Dossier des doublons",
		"duplicate_section":    "Tâches de détection des doublons",
		"duplicate_job":        "Tâche de doublons",
		"enabled":              "Activé",
		"save_target":          "Destination d'enregistrement",
		"save_target_active":   "Enregistrer dans le fichier de configuration actif et appliquer",
		"save_target_separate": "Enregistrer dans un fichier séparé",
		"separate_path":        "Chemin du fichier séparé",
		"validate":             "Valider",
		"save":                 "Enregistrer",
		"language":             "Langue",
		"success_prefix":       "Enregistré avec succès dans",
		"validation_failed":    "Validation échouée",
		"save_failed":          "Échec de l'enregistrement",
		"private_notice":       "Prévu pour une utilisation locale/privée. Ne pas exposer publiquement sans protection supplémentaire.",
	},
}

func normalizeLanguage(raw string) string {
	lang := strings.TrimSpace(strings.ToLower(raw))
	if len(lang) >= 2 {
		lang = lang[:2]
	}
	if _, ok := supportedLanguages[lang]; ok {
		return lang
	}
	return defaultLanguage
}

func detectLanguage(queryLang string, cookieLang string, acceptLanguage string) string {
	if lang := normalizeLanguage(queryLang); lang != defaultLanguage || strings.HasPrefix(strings.ToLower(strings.TrimSpace(queryLang)), defaultLanguage) {
		if _, ok := supportedLanguages[lang]; ok && queryLang != "" {
			return lang
		}
	}
	if cookieLang != "" {
		return normalizeLanguage(cookieLang)
	}
	for _, part := range strings.Split(acceptLanguage, ",") {
		if lang := normalizeLanguage(part); lang != defaultLanguage || strings.Contains(strings.ToLower(part), defaultLanguage) {
			if _, ok := supportedLanguages[lang]; ok {
				return lang
			}
		}
	}
	return defaultLanguage
}

func t(lang string, key string) string {
	lang = normalizeLanguage(lang)
	if value, ok := supportedLanguages[lang][key]; ok {
		return value
	}
	if value, ok := supportedLanguages[defaultLanguage][key]; ok {
		return value
	}
	return key
}
