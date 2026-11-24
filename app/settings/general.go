package settings

import (
	"github.com/wieku/danser-go/framework/env"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var General = initGeneral()

func initGeneral() *general {
	osuBaseDir := getOsuInstallation()

	return &general{
		OsuSongsDir:       filepath.Join(osuBaseDir, "Songs"),
		OsuSkinsDir:       filepath.Join(osuBaseDir, "Skins"),
		OsuReplaysDir:     filepath.Join(osuBaseDir, "Replays"),
		LazerMode:         false,
		LazerPath:         "",
		DiscordPresenceOn: true,
		UnpackOszFiles:    true,
		VerboseImportLogs: false,
	}
}

type general struct {
	// Directory that contains osu! songs
	OsuSongsDir string `long:"true" label:"osu! Songs directory" path:"Select osu! Songs directory"`

	// Directory that contains osu! skins
	OsuSkinsDir string `long:"true" label:"osu! Skins directory" path:"Select osu! Skins directory"`

	// Directory that contains osu! replays
	OsuReplaysDir string `long:"true" label:"osu! Replays directory" path:"Select osu! Replays directory" tooltip:"Don't use replays directory inside danser's directory!"`

	// Whether danser should read beatmaps and replays directly from osu!lazer storage
	LazerMode bool `label:"osu!lazer mode"`

	// Directory that contains osu!lazer data (defaults to %AppData%\osu when empty)
	LazerPath string `long:"true" label:"osu!lazer directory" path:"Select osu!lazer directory" tooltip:"Folder containing client.realm (leave empty for %AppData%\\osu)"`

	// Whether discord should show that danser is on
	DiscordPresenceOn bool `label:"Discord Rich Presence"`

	// Whether danser should unpack .osz files in Songs folder, osu! may complain about it
	UnpackOszFiles bool

	// Whether import details should be shown. If false, only failures will be logged.
	VerboseImportLogs bool

	songsDir   *string
	skinsDir   *string
	replaysDir *string
}

func (g *general) GetSongsDir() string {
	if g.songsDir == nil {
		dir := filepath.Join(env.DataDir(), g.OsuSongsDir)

		if filepath.IsAbs(g.OsuSongsDir) {
			dir = g.OsuSongsDir
		}

		g.songsDir = &dir
	}

	return *g.songsDir
}

func (g *general) GetSkinsDir() string {
	if g.skinsDir == nil {
		dir := filepath.Join(env.DataDir(), g.OsuSkinsDir)

		if filepath.IsAbs(g.OsuSkinsDir) {
			dir = g.OsuSkinsDir
		}

		g.skinsDir = &dir
	}

	return *g.skinsDir
}

func (g *general) GetReplaysDir() string {
	if g.replaysDir == nil {
		dir := filepath.Join(env.DataDir(), g.OsuReplaysDir)

		if filepath.IsAbs(g.OsuReplaysDir) {
			dir = g.OsuReplaysDir
		}

		g.replaysDir = &dir
	}

	return *g.replaysDir
}

func (g *general) GetLazerPath() string {
	trimmed := strings.TrimSpace(g.LazerPath)
	if trimmed == "" {
		return getDefaultLazerPath()
	}

	resolved := trimmed
	if !filepath.IsAbs(trimmed) {
		resolved = filepath.Join(env.DataDir(), trimmed)
	}

	if _, err := os.Stat(filepath.Join(resolved, "client.realm")); os.IsNotExist(err) {
		defaultDir := getDefaultLazerPath()
		if _, defErr := os.Stat(filepath.Join(defaultDir, "client.realm")); defErr == nil {
			log.Printf("SettingsManager: client.realm not found in LazerPath (%s), falling back to %s\n", resolved, defaultDir)
			return defaultDir
		}
	}

	return resolved
}
