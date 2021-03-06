package oot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"kaepora/internal/generator"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

const RandomizerName = "oot-randomizer"

// Randomizer is the local OOTR generator.
type Randomizer struct {
	version string
}

// State is the generator-specific state carried along OOT seeds.
type State struct {
	// For API-generated seeds only.
	ID string `json:",omitempty"`

	SettingsPatch map[string]interface{} `json:",omitempty"`
}

func NewRandomizer(version string) *Randomizer {
	return &Randomizer{
		version: version,
	}
}

func (g *Randomizer) Generate(settingsName, seed string) (generator.Output, error) {
	outDir, err := ioutil.TempDir("", "oot-randomizer-output-")
	if err != nil {
		return generator.Output{}, fmt.Errorf("unable to create output directory: %s", err)
	}
	defer os.RemoveAll(outDir)

	base, err := GetBaseDir()
	if err != nil {
		return generator.Output{}, err
	}
	settingsPath := filepath.Join(base, settingsName)

	zpf, spoilerLog, err := g.run(outDir, settingsPath, seed)
	if err != nil {
		return generator.Output{}, fmt.Errorf("unable to generate seed: %s", err)
	}

	settings, err := readSettingsFile(settingsPath)
	if err != nil {
		return generator.Output{}, fmt.Errorf("unable to read settings: %w", err)
	}
	state, err := patchStateWithSettings(nil, settings)
	if err != nil {
		return generator.Output{}, fmt.Errorf("unable to patch state with settings: %w", err)
	}

	return generator.Output{
		State:      state,
		SeedPatch:  zpf,
		SpoilerLog: spoilerLog,
	}, nil
}

func readFirstGlob(pattern string) ([]byte, error) {
	names, err := filepath.Glob(pattern)
	if err != nil || len(names) != 1 {
		return nil, fmt.Errorf("could not find file with glob `%s`: %w", pattern, err)
	}

	out, err := ioutil.ReadFile(names[0])
	if err != nil {
		return nil, fmt.Errorf("unable to read seed back: %w", err)
	}

	return out, nil
}

// GetBaseDir returns the directory where all the resources needed by the
// generators are stored.
func GetBaseDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Join(wd, "resources/oot-randomizer"), nil
}

func (g *Randomizer) run(outDir, settings, seed string) ([]byte, []byte, error) {
	base, err := GetBaseDir()
	if err != nil {
		return nil, nil, err
	}

	user, err := user.Current()
	if err != nil {
		return nil, nil, err
	}

	// There's no user input, unless the DB has been taken over.
	// nolint: gosec
	cmd := exec.Command(
		"docker", "run", "--rm",
		"-u", fmt.Sprintf("%s:%s", user.Uid, user.Gid),
		"-v", base+"/ARCHIVE.bin:/opt/oot-randomizer/ARCHIVE.bin:ro",
		"-v", base+"/ZOOTDEC.z64:/opt/oot-randomizer/ZOOTDEC.z64:ro",
		"-v", settings+":/opt/oot-randomizer/settings.json:ro",
		"-v", outDir+":/opt/oot-randomizer/Output",
		"lp042/oot-randomizer:"+g.version,
		"--seed", seed,
		"--settings", "settings.json",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("stdout: %s", stdout.String())
		log.Printf("stderr: %s", stderr.String())
		return nil, nil, err
	}

	zpf, err := readFirstGlob(filepath.Join(outDir, "*.zpf"))
	if err != nil {
		return nil, nil, err
	}

	spoilerLog, err := readFirstGlob(filepath.Join(outDir, "*_Spoiler.json"))
	if err != nil {
		return nil, nil, err
	}

	return zpf, spoilerLog, nil
}

func (g *Randomizer) GetDownloadURL([]byte) string {
	return ""
}

func (g *Randomizer) IsExternal() bool {
	return false
}

func (g *Randomizer) UnlockSpoilerLog([]byte) error {
	return nil
}

func readSettingsFile(path string) (map[string]interface{}, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(r)

	var settings map[string]interface{}
	if err := dec.Decode(&settings); err != nil {
		return nil, err
	}

	return settings, nil
}
