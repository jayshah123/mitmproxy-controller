package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultProfileID = "default"
)

type ServiceProfile struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Scripts     []string          `yaml:"scripts"`
	SetOptions  map[string]string `yaml:"set_options"`
	Mode        string            `yaml:"mode,omitempty"`
	FilePath    string            `yaml:"-"`
	ScriptPaths []string          `yaml:"-"`
	Warnings    []string          `yaml:"-"`
	ProxyCompat bool              `yaml:"-"`
	WebUICompat bool              `yaml:"-"`
}

type profileFile struct {
	ID         string                 `yaml:"id"`
	Name       string                 `yaml:"name"`
	Scripts    []string               `yaml:"scripts"`
	SetOptions map[string]interface{} `yaml:"set_options"`
	Mode       string                 `yaml:"mode"`
}

type controllerState struct {
	SelectedProfileID string `json:"selected_profile_id"`
}

var (
	serviceProfiles   []ServiceProfile
	selectedProfileID = defaultProfileID
	profileWarnings   []string
)

func getControllerDataDirectory() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}
	return filepath.Join(configDir, "mitmproxy-controller")
}

func getProfilesDirectory() string {
	return filepath.Join(getControllerDataDirectory(), "profiles")
}

func getStatePath() string {
	return filepath.Join(getControllerDataDirectory(), "state.json")
}

func initProfiles() error {
	if err := loadProfilesFromDisk(); err != nil {
		return err
	}
	stateID := loadSelectedProfileID()
	if hasProfile(stateID) {
		selectedProfileID = stateID
	} else {
		selectedProfileID = defaultProfileID
		_ = saveSelectedProfileID(selectedProfileID)
	}
	return nil
}

func loadProfilesFromDisk() error {
	profiles, warnings, err := discoverProfiles()
	if err != nil {
		return err
	}
	serviceProfiles = profiles
	profileWarnings = warnings

	if !hasProfile(selectedProfileID) {
		selectedProfileID = defaultProfileID
	}
	return nil
}

func discoverProfiles() ([]ServiceProfile, []string, error) {
	if err := ensureProfilesDirectory(); err != nil {
		return nil, nil, err
	}

	entries, err := os.ReadDir(getProfilesDirectory())
	if err != nil {
		return nil, nil, err
	}

	profiles := make([]ServiceProfile, 0, len(entries))
	warnings := make([]string, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		filePath := filepath.Join(getProfilesDirectory(), entry.Name())
		profile, err := loadProfileFile(filePath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", entry.Name(), err))
			continue
		}
		profiles = append(profiles, profile)
	}

	if len(profiles) == 0 {
		profiles = append(profiles, makeFallbackDefaultProfile())
	}

	sortProfiles(profiles)
	return profiles, warnings, nil
}

func ensureProfilesDirectory() error {
	profilesDir := getProfilesDirectory()
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		return err
	}

	defaultPath := filepath.Join(profilesDir, defaultProfileID+".yaml")
	if _, err := os.Stat(defaultPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	defaultProfileTemplate := `id: default
name: Default
scripts: []
set_options: {}
`
	return os.WriteFile(defaultPath, []byte(defaultProfileTemplate), 0644)
}

func loadProfileFile(filePath string) (ServiceProfile, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ServiceProfile{}, err
	}

	var parsed profileFile
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		return ServiceProfile{}, err
	}

	id := strings.TrimSpace(parsed.ID)
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	id = sanitizeProfileID(id)
	if id == "" {
		return ServiceProfile{}, fmt.Errorf("profile id is empty")
	}

	name := strings.TrimSpace(parsed.Name)
	if name == "" {
		name = id
	}

	p := ServiceProfile{
		ID:         id,
		Name:       name,
		Scripts:    normalizeStringSlice(parsed.Scripts),
		SetOptions: make(map[string]string),
		Mode:       strings.TrimSpace(parsed.Mode),
		FilePath:   filePath,
	}

	for key, value := range parsed.SetOptions {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		p.SetOptions[key] = optionValueToString(value)
	}

	populateProfileDerivedFields(&p)
	return p, nil
}

func makeFallbackDefaultProfile() ServiceProfile {
	p := ServiceProfile{
		ID:         defaultProfileID,
		Name:       "Default",
		Scripts:    []string{},
		SetOptions: map[string]string{},
		FilePath:   filepath.Join(getProfilesDirectory(), defaultProfileID+".yaml"),
	}
	populateProfileDerivedFields(&p)
	return p
}

func populateProfileDerivedFields(profile *ServiceProfile) {
	profile.ScriptPaths = make([]string, 0, len(profile.Scripts))
	profile.Warnings = make([]string, 0)
	profile.ProxyCompat = true
	profile.WebUICompat = true

	baseDir := filepath.Dir(profile.FilePath)
	for _, script := range profile.Scripts {
		scriptPath := script
		if !filepath.IsAbs(scriptPath) {
			scriptPath = filepath.Join(baseDir, scriptPath)
		}
		absPath, err := filepath.Abs(scriptPath)
		if err == nil {
			scriptPath = absPath
		}
		profile.ScriptPaths = append(profile.ScriptPaths, scriptPath)

		if _, err := os.Stat(scriptPath); err != nil {
			profile.Warnings = append(profile.Warnings, fmt.Sprintf("missing script: %s", scriptPath))
		}
	}

	profile.ProxyCompat = isOptionCompatible(profile.SetOptions, "listen_host", proxyHost) &&
		isOptionCompatible(profile.SetOptions, "listen_port", proxyPort)
	profile.WebUICompat = isOptionCompatible(profile.SetOptions, "web_host", proxyHost) &&
		isOptionCompatible(profile.SetOptions, "web_port", webUIPort) &&
		isOptionCompatible(profile.SetOptions, "web_password", webPassword)

	if !profile.ProxyCompat {
		profile.Warnings = append(profile.Warnings, "proxy actions disabled (listen_host/listen_port override)")
	}
	if !profile.WebUICompat {
		profile.Warnings = append(profile.Warnings, "web UI action disabled (web host/port/password override)")
	}
	if _, hasConfdir := profile.SetOptions["confdir"]; hasConfdir {
		profile.Warnings = append(profile.Warnings, "confdir override ignored in profile set_options")
	}
}

func sanitizeProfileID(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "_", "-")
	normalized = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-':
			return r
		default:
			return -1
		}
	}, normalized)
	return normalized
}

func normalizeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func optionValueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	default:
		return fmt.Sprint(v)
	}
}

func sortProfiles(profiles []ServiceProfile) {
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].ID == defaultProfileID {
			return true
		}
		if profiles[j].ID == defaultProfileID {
			return false
		}
		return strings.ToLower(profiles[i].Name) < strings.ToLower(profiles[j].Name)
	})
}

func loadSelectedProfileID() string {
	statePath := getStatePath()
	content, err := os.ReadFile(statePath)
	if err != nil {
		return defaultProfileID
	}

	var state controllerState
	if err := json.Unmarshal(content, &state); err != nil {
		return defaultProfileID
	}

	id := sanitizeProfileID(state.SelectedProfileID)
	if id == "" {
		return defaultProfileID
	}
	return id
}

func saveSelectedProfileID(profileID string) error {
	if err := os.MkdirAll(getControllerDataDirectory(), 0755); err != nil {
		return err
	}

	state := controllerState{SelectedProfileID: profileID}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getStatePath(), payload, 0644)
}

func listProfiles() []ServiceProfile {
	out := make([]ServiceProfile, len(serviceProfiles))
	copy(out, serviceProfiles)
	return out
}

func hasProfile(profileID string) bool {
	for _, p := range serviceProfiles {
		if p.ID == profileID {
			return true
		}
	}
	return false
}

func getSelectedProfile() (ServiceProfile, bool) {
	for _, p := range serviceProfiles {
		if p.ID == selectedProfileID {
			return p, true
		}
	}
	return ServiceProfile{}, false
}

func setSelectedProfile(profileID string) error {
	profileID = sanitizeProfileID(profileID)
	if profileID == "" {
		return fmt.Errorf("invalid profile id")
	}
	if !hasProfile(profileID) {
		return fmt.Errorf("profile %q not found", profileID)
	}
	selectedProfileID = profileID
	return saveSelectedProfileID(profileID)
}

func getProfileByID(profileID string) (ServiceProfile, bool) {
	for _, p := range serviceProfiles {
		if p.ID == profileID {
			return p, true
		}
	}
	return ServiceProfile{}, false
}

func selectedProfileName() string {
	p, ok := getSelectedProfile()
	if !ok {
		return "Unknown"
	}
	return p.Name
}

func selectedProfileWarnings() []string {
	p, ok := getSelectedProfile()
	if !ok {
		return nil
	}
	out := make([]string, len(p.Warnings))
	copy(out, p.Warnings)
	return out
}

func selectedProfilePath() string {
	p, ok := getSelectedProfile()
	if !ok {
		return ""
	}
	return p.FilePath
}

func selectedProfileScriptsFolder() string {
	p, ok := getSelectedProfile()
	if !ok {
		return getProfilesDirectory()
	}
	if len(p.ScriptPaths) > 0 {
		return filepath.Dir(p.ScriptPaths[0])
	}
	return filepath.Dir(p.FilePath)
}

func ensureSelectedProfileScriptsFolder() (string, error) {
	folder := selectedProfileScriptsFolder()
	if folder == "" {
		folder = getProfilesDirectory()
	}
	abs, err := filepath.Abs(folder)
	if err == nil {
		folder = abs
	}
	if err := os.MkdirAll(folder, 0755); err != nil {
		return "", err
	}
	return folder, nil
}

func selectedProfileCompatibility() (proxyCompatible bool, webUICompatible bool) {
	p, ok := getSelectedProfile()
	if !ok {
		return true, true
	}
	return p.ProxyCompat, p.WebUICompat
}

func isOptionCompatible(options map[string]string, key, expected string) bool {
	value, ok := options[key]
	if !ok {
		return true
	}
	return strings.TrimSpace(value) == expected
}

func profileLoadWarnings() []string {
	out := make([]string, len(profileWarnings))
	copy(out, profileWarnings)
	return out
}
