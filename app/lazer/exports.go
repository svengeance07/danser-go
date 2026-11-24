package lazer

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wieku/danser-go/app/settings"
)

var exportDateRegex = regexp.MustCompile(`\((\d{4}-\d{2}-\d{2}_\d{2}-\d{2})\)\.osr$`)

// ExportedReplay represents a .osr file inside osu!lazer's exports directory.
type ExportedReplay struct {
	Name     string
	Path     string
	Date     time.Time
	DateText string
}

// ParseExportReplayDate extracts the timestamp from an osu!lazer exported replay filename,
// falling back to the provided file modification time if the suffix is missing.
func ParseExportReplayDate(filename string, fallback time.Time) time.Time {
	matches := exportDateRegex.FindStringSubmatch(filename)
	if len(matches) == 2 {
		if parsed, err := time.ParseInLocation("2006-01-02_15-04", matches[1], time.Local); err == nil {
			return parsed
		}
	}

	return fallback
}

// ListExportedReplays returns .osr files in <LazerPath>/exports sorted by date.
func ListExportedReplays(newestFirst bool) ([]ExportedReplay, error) {
	exportsDir := filepath.Join(settings.General.GetLazerPath(), "exports")
	entries, err := os.ReadDir(exportsDir)
	if err != nil {
		return nil, err
	}

	replays := make([]ExportedReplay, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".osr") {
			continue
		}

		var modTime time.Time
		if info, infoErr := entry.Info(); infoErr == nil {
			modTime = info.ModTime()
		}

		date := ParseExportReplayDate(entry.Name(), modTime)
		baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

		dateText := ""
		if !date.IsZero() {
			dateText = date.Format("2006-01-02 15:04")
		}

		replays = append(replays, ExportedReplay{
			Name:     baseName,
			Path:     filepath.Join(exportsDir, entry.Name()),
			Date:     date,
			DateText: dateText,
		})
	}

	sort.SliceStable(replays, func(i, j int) bool {
		if !replays[i].Date.Equal(replays[j].Date) {
			if newestFirst {
				return replays[i].Date.After(replays[j].Date)
			}
			return replays[i].Date.Before(replays[j].Date)
		}

		return strings.ToLower(replays[i].Name) < strings.ToLower(replays[j].Name)
	})

	return replays, nil
}
