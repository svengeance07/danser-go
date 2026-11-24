package lazer

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/rplpa"
)

func TestLazerDefaultsDisabled(t *testing.T) {
	if settings.General.LazerMode {
		t.Fatal("expected LazerMode to default to false to preserve osu!stable compatibility")
	}
	if settings.General.GetLazerPath() == "" {
		t.Fatal("expected GetLazerPath() to return a non-empty default path")
	}
}

func TestLazerBridgeIntegration(t *testing.T) {
	lazerDir := settings.General.GetLazerPath()
	realmPath := filepath.Join(lazerDir, "client.realm")
	if _, err := os.Stat(realmPath); os.IsNotExist(err) {
		t.Skipf("skipping integration test: %s not found", realmPath)
	}

	if _, err := FindBridgeExecutable(); err != nil {
		t.Fatalf("FindBridgeExecutable() failed: %v", err)
	}

	replays, err := ListReplays(5)
	if err != nil {
		t.Fatalf("ListReplays(5) failed: %v", err)
	}

	if len(replays) == 0 {
		t.Skip("no osu!standard replays found in client.realm")
	}

	var loaded *beatmap.BeatMap
	for _, r := range replays {
		if r.BeatmapMD5 == "" {
			continue
		}
		if _, err := os.Stat(r.ReplayPath); err != nil {
			t.Fatalf("replay file missing on disk %s: %v", r.ReplayPath, err)
		}

		replayBytes, err := os.ReadFile(r.ReplayPath)
		if err != nil {
			t.Fatalf("failed to read replay file %s: %v", r.ReplayPath, err)
		}
		if _, err := rplpa.ParseReplay(replayBytes); err != nil {
			t.Fatalf("rplpa.ParseReplay(%s) failed: %v", r.ReplayPath, err)
		}

		bMap, err := LoadBeatmapByMD5(r.BeatmapMD5)
		if err != nil {
			continue
		}
		loaded = bMap
		break
	}

	if loaded == nil {
		t.Skip("none of the top 5 replays had a locally available beatmap in client.realm")
	}

	if loaded.GetOsuFilePath() == "" {
		t.Fatal("loaded beatmap has empty OsuFilePath")
	}
	if _, err := os.Stat(loaded.GetOsuFilePath()); err != nil {
		t.Fatalf(".osu file missing on disk %s: %v", loaded.GetOsuFilePath(), err)
	}

	if loaded.Audio != "" {
		audioPath, err := loaded.GetAudioFile()
		if err != nil {
			t.Fatalf("GetAudioFile(%q) failed: %v", loaded.Audio, err)
		}
		if _, err := os.Stat(audioPath); err != nil {
			t.Fatalf("audio file missing on disk %s: %v", audioPath, err)
		}
	}

	if loaded.Bg != "" {
		bgPath, err := loaded.GetBgFile()
		if err != nil {
			t.Fatalf("GetBgFile(%q) failed: %v", loaded.Bg, err)
		}
		if _, err := os.Stat(bgPath); err != nil {
			t.Fatalf("background file missing on disk %s: %v", bgPath, err)
		}
	}
}

func TestListExportedReplaysSorted(t *testing.T) {
	fallback := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	parsed := ParseExportReplayDate("player playing Song [Diff] (2026-11-22_19-24).osr", fallback)
	want := time.Date(2026, 11, 22, 19, 24, 0, 0, time.Local)
	if !parsed.Equal(want) {
		t.Fatalf("ParseExportReplayDate = %v, want %v", parsed, want)
	}

	if got := ParseExportReplayDate("no_timestamp.osr", fallback); !got.Equal(fallback) {
		t.Fatalf("ParseExportReplayDate fallback = %v, want %v", got, fallback)
	}

	exportsDir := filepath.Join(settings.General.GetLazerPath(), "exports")
	if _, err := os.Stat(exportsDir); os.IsNotExist(err) {
		t.Skipf("skipping exports integration test: %s not found", exportsDir)
	}

	newestFirst, err := ListExportedReplays(true)
	if err != nil {
		t.Fatalf("ListExportedReplays(true) failed: %v", err)
	}
	for i := 1; i < len(newestFirst); i++ {
		if newestFirst[i].Date.After(newestFirst[i-1].Date) {
			t.Fatalf("newestFirst out of order at %d: %v > %v", i, newestFirst[i].Date, newestFirst[i-1].Date)
		}
	}

	oldestFirst, err := ListExportedReplays(false)
	if err != nil {
		t.Fatalf("ListExportedReplays(false) failed: %v", err)
	}
	for i := 1; i < len(oldestFirst); i++ {
		if oldestFirst[i].Date.Before(oldestFirst[i-1].Date) {
			t.Fatalf("oldestFirst out of order at %d: %v < %v", i, oldestFirst[i].Date, oldestFirst[i-1].Date)
		}
	}
}

func TestLazerSkinsIntegration(t *testing.T) {
	realmPath := filepath.Join(settings.General.GetLazerPath(), "client.realm")
	if _, err := os.Stat(realmPath); os.IsNotExist(err) {
		t.Skipf("skipping skin integration test: %s not found", realmPath)
	}

	skins, err := ListSkins()
	if err != nil {
		t.Fatalf("ListSkins() failed: %v", err)
	}
	if len(skins) == 0 {
		t.Skip("no custom skins found in client.realm")
	}

	fm, err := LoadSkinFileMap(skins[0].Name)
	if err != nil {
		t.Fatalf("LoadSkinFileMap(%q) failed: %v", skins[0].Name, err)
	}
	if len(fm.GetMap()) == 0 {
		t.Fatalf("LoadSkinFileMap(%q) returned empty FileMap", skins[0].Name)
	}

	for name, physPath := range fm.GetMap() {
		if _, err := os.Stat(physPath); err != nil {
			t.Fatalf("skin file %q missing on disk at %s: %v", name, physPath, err)
		}
		break
	}
}
