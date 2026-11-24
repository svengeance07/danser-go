package launcher

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/wieku/danser-go/app/lazer"
)

type lazerReplayPopup struct {
	*popup

	exports    []lazer.ExportedReplay
	exportsErr error

	replays    []lazer.LazerReplay
	replaysErr error

	newestFirst bool
	searchText  string
	onPick      func(replayPath string)
}

func newLazerReplayPopup(onPick func(replayPath string)) *lazerReplayPopup {
	p := &lazerReplayPopup{
		popup:       newPopup("Select osu!lazer replay", popBig),
		newestFirst: true,
		onPick:      onPick,
	}

	p.internalDraw = p.drawList
	p.refresh()

	return p
}

func (p *lazerReplayPopup) refresh() {
	p.exports, p.exportsErr = lazer.ListExportedReplays(p.newestFirst)
	p.replays, p.replaysErr = lazer.ListReplays(50)
	p.sortLists()
}

func (p *lazerReplayPopup) sortLists() {
	sort.SliceStable(p.exports, func(i, j int) bool {
		if !p.exports[i].Date.Equal(p.exports[j].Date) {
			if p.newestFirst {
				return p.exports[i].Date.After(p.exports[j].Date)
			}
			return p.exports[i].Date.Before(p.exports[j].Date)
		}
		return strings.ToLower(p.exports[i].Name) < strings.ToLower(p.exports[j].Name)
	})

	sort.SliceStable(p.replays, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339Nano, p.replays[i].Date)
		tj, _ := time.Parse(time.RFC3339Nano, p.replays[j].Date)
		if !ti.Equal(tj) {
			if p.newestFirst {
				return ti.After(tj)
			}
			return ti.Before(tj)
		}
		return strings.ToLower(p.replays[i].Title) < strings.ToLower(p.replays[j].Title)
	})
}

func (p *lazerReplayPopup) pick(replayPath string) {
	p.opened = false
	imgui.CloseCurrentPopup()
	if p.onPick != nil {
		p.onPick(replayPath)
	}
}

func (p *lazerReplayPopup) drawList() {
	imgui.PushFont(Font, 20)
	defer imgui.PopFont()

	if imgui.Button("Refresh") {
		p.refresh()
	}

	imgui.SameLine()
	sortLabel := "Sort: Newest first"
	if !p.newestFirst {
		sortLabel = "Sort: Oldest first"
	}
	if imgui.Button(sortLabel) {
		p.newestFirst = !p.newestFirst
		p.sortLists()
	}

	imgui.SameLine()
	imgui.SetNextItemWidth(-1)
	imgui.InputTextWithHint("##lazer_replay_filter", "Filter by map, player, or date...", &p.searchText, imgui.InputTextFlagsNone, nil)

	imgui.Separator()

	filter := strings.ToLower(strings.TrimSpace(p.searchText))

	if imgui.BeginTabBar("lazer_replays_tabs") {
		if imgui.BeginTabItem("Exports") {
			p.drawExportsTab(filter)
			imgui.EndTabItem()
		}

		if imgui.BeginTabItem("Local scores") {
			p.drawLocalScoresTab(filter)
			imgui.EndTabItem()
		}

		imgui.EndTabBar()
	}
}

func (p *lazerReplayPopup) drawExportsTab(filter string) {
	if p.exportsErr != nil {
		imgui.TextWrapped(fmt.Sprintf("Failed to read exports folder: %v", p.exportsErr))
		return
	}

	if len(p.exports) == 0 {
		imgui.TextUnformatted("No .osr files found in osu!lazer exports folder.")
		return
	}

	avail := imgui.ContentRegionAvail()
	if imgui.BeginChildStrV("lazer_exports_list", vec2(avail.X, avail.Y-5), imgui.ChildFlagsBorders, 0) {
		for i, exp := range p.exports {
			label := exp.Name
			if exp.DateText != "" {
				label = fmt.Sprintf("%s | %s", exp.DateText, exp.Name)
			}

			if filter != "" && !strings.Contains(strings.ToLower(label), filter) {
				continue
			}

			if imgui.SelectableBool(fmt.Sprintf("%s##exp_%d", label, i)) {
				p.pick(exp.Path)
			}
		}
		imgui.EndChild()
	}
}

func (p *lazerReplayPopup) drawLocalScoresTab(filter string) {
	if p.replaysErr != nil {
		imgui.TextWrapped(fmt.Sprintf("Failed to load osu!lazer replays: %v", p.replaysErr))
		return
	}

	if len(p.replays) == 0 {
		imgui.TextUnformatted("No osu!standard replays found in client.realm.")
		return
	}

	avail := imgui.ContentRegionAvail()
	if imgui.BeginChildStrV("lazer_replays_list", vec2(avail.X, avail.Y-5), imgui.ChildFlagsBorders, 0) {
		for i, r := range p.replays {
			title := fmt.Sprintf("%s - %s [%s]", r.Artist, r.Title, r.Difficulty)
			if r.Artist == "" && r.Title == "" {
				title = r.BeatmapMD5
			}

			dateText := ""
			if parsed, err := time.Parse(time.RFC3339Nano, r.Date); err == nil {
				dateText = parsed.Local().Format("2006-01-02 15:04") + " | "
			}

			label := fmt.Sprintf("%s%s | %s | %.2f%% | %dx", dateText, title, r.Username, r.Accuracy*100, r.MaxCombo)
			if filter != "" && !strings.Contains(strings.ToLower(label), filter) {
				continue
			}

			if imgui.SelectableBool(fmt.Sprintf("%s##lzr_%d", label, i)) {
				p.pick(r.ReplayPath)
			}
		}
		imgui.EndChild()
	}
}
