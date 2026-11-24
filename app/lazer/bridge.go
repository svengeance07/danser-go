package lazer

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/wieku/danser-go/app/beatmap"
	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/framework/env"
	"github.com/wieku/danser-go/framework/files"
)

// ResolvedBeatmap represents the JSON response from `lazer-bridge resolve-beatmap`.
type ResolvedBeatmap struct {
	MD5                string            `json:"md5"`
	Hash               string            `json:"hash"`
	OsuPath            string            `json:"osuPath"`
	AudioPath          string            `json:"audioPath"`
	BackgroundPath     string            `json:"backgroundPath"`
	AudioFilename      string            `json:"audioFilename"`
	BackgroundFilename string            `json:"backgroundFilename"`
	Artist             string            `json:"artist"`
	Title              string            `json:"title"`
	Difficulty         string            `json:"difficulty"`
	Creator            string            `json:"creator"`
	Files              map[string]string `json:"files"`
}

// LazerReplay represents a replay entry returned by `lazer-bridge list-replays`.
type LazerReplay struct {
	Hash        string  `json:"hash"`
	BeatmapMD5  string  `json:"beatmapMd5"`
	BeatmapHash string  `json:"beatmapHash"`
	ReplayPath  string  `json:"replayPath"`
	Username    string  `json:"username"`
	Artist      string  `json:"artist"`
	Title       string  `json:"title"`
	Difficulty  string  `json:"difficulty"`
	TotalScore  int64   `json:"totalScore"`
	MaxCombo    int     `json:"maxCombo"`
	Accuracy    float64 `json:"accuracy"`
	Date        string  `json:"date"`
	Mods        string  `json:"mods"`
}

var ErrBridgeNotFound = errors.New("lazer-bridge executable not found")

// FindBridgeExecutable locates lazer-bridge(.exe) next to the running binary,
// in the current working directory, in danser's lib/data directories, or in PATH.
func FindBridgeExecutable() (string, error) {
	binName := "lazer-bridge"
	if runtime.GOOS == "windows" {
		binName = "lazer-bridge.exe"
	}

	candidates := make([]string, 0, 6)

	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), binName))
	}

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, binName))
	}

	for _, dir := range safeEnvDirs() {
		if dir != "" {
			candidates = append(candidates, filepath.Join(dir, binName))
		}
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	if pathExec, err := exec.LookPath(binName); err == nil {
		return pathExec, nil
	}

	return "", ErrBridgeNotFound
}

func safeEnvDirs() (dirs []string) {
	defer func() {
		_ = recover()
	}()

	dirs = append(dirs, env.LibDir(), env.DataDir())
	return dirs
}

func runBridge(args ...string) ([]byte, error) {
	bridgePath, err := FindBridgeExecutable()
	if err != nil {
		return nil, err
	}

	if lazerDir := strings.TrimSpace(settings.General.GetLazerPath()); lazerDir != "" {
		args = append(args, "--lazer-path", lazerDir)
	}

	cmd := exec.Command(bridgePath, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("lazer-bridge %s failed: %s", args[0], strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("lazer-bridge %s failed: %w", args[0], err)
	}

	return out, nil
}

// ResolveBeatmapByMD5 queries lazer-bridge for a beatmap matching the given MD5 hash.
func ResolveBeatmapByMD5(md5Hash string) (*ResolvedBeatmap, error) {
	if strings.TrimSpace(md5Hash) == "" {
		return nil, errors.New("empty beatmap md5")
	}

	out, err := runBridge("resolve-beatmap", "--md5", md5Hash)
	if err != nil {
		return nil, err
	}

	var resolved ResolvedBeatmap
	if err := json.Unmarshal(out, &resolved); err != nil {
		return nil, fmt.Errorf("failed to decode resolve-beatmap output: %w", err)
	}

	return &resolved, nil
}

// ResolveBeatmapByHash queries lazer-bridge for a beatmap matching the given SHA-256 hash.
func ResolveBeatmapByHash(sha256Hash string) (*ResolvedBeatmap, error) {
	if strings.TrimSpace(sha256Hash) == "" {
		return nil, errors.New("empty beatmap hash")
	}

	out, err := runBridge("resolve-beatmap", "--hash", sha256Hash)
	if err != nil {
		return nil, err
	}

	var resolved ResolvedBeatmap
	if err := json.Unmarshal(out, &resolved); err != nil {
		return nil, fmt.Errorf("failed to decode resolve-beatmap output: %w", err)
	}

	return &resolved, nil
}

// ListReplays queries lazer-bridge for the most recent local osu!standard replays.
func ListReplays(limit int) ([]LazerReplay, error) {
	if limit <= 0 {
		limit = 20
	}

	out, err := runBridge("list-replays", "--limit", strconv.Itoa(limit))
	if err != nil {
		return nil, err
	}

	var replays []LazerReplay
	if err := json.Unmarshal(out, &replays); err != nil {
		return nil, fmt.Errorf("failed to decode list-replays output: %w", err)
	}

	return replays, nil
}

// LoadBeatmapByMD5 resolves a beatmap from osu!lazer storage and parses it with a virtual FileMap.
func LoadBeatmapByMD5(md5Hash string) (*beatmap.BeatMap, error) {
	resolved, err := ResolveBeatmapByMD5(md5Hash)
	if err != nil {
		return nil, err
	}

	if resolved.OsuPath == "" {
		return nil, fmt.Errorf("resolved beatmap %s has no .osu file path", md5Hash)
	}

	fileEntries := make(map[string]string, len(resolved.Files)+2)
	for name, phys := range resolved.Files {
		fileEntries[name] = phys
	}

	if resolved.AudioFilename != "" && resolved.AudioPath != "" {
		if _, exists := fileEntries[resolved.AudioFilename]; !exists {
			fileEntries[resolved.AudioFilename] = resolved.AudioPath
		}
	}

	if resolved.BackgroundFilename != "" && resolved.BackgroundPath != "" {
		if _, exists := fileEntries[resolved.BackgroundFilename]; !exists {
			fileEntries[resolved.BackgroundFilename] = resolved.BackgroundPath
		}
	}

	vMap := files.NewVirtualFileMap(fileEntries)
	bMap, err := beatmap.ParseBeatMapFromPath(resolved.OsuPath, vMap)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lazer beatmap %s: %w", resolved.OsuPath, err)
	}

	if resolved.MD5 != "" {
		bMap.MD5 = strings.ToLower(resolved.MD5)
	} else if f, openErr := os.Open(resolved.OsuPath); openErr == nil {
		h := md5.New()
		if _, copyErr := io.Copy(h, f); copyErr == nil {
			bMap.MD5 = hex.EncodeToString(h.Sum(nil))
		}
		f.Close()
	}

	if bMap.Audio == "" && resolved.AudioFilename != "" {
		bMap.Audio = resolved.AudioFilename
	}

	if bMap.Bg == "" && resolved.BackgroundFilename != "" {
		bMap.Bg = resolved.BackgroundFilename
	}

	return bMap, nil
}
