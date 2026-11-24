package settings

import (
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
)

var Skin = initSkin()

func initSkin() *skin {
	return &skin{
		CurrentSkin:       "default",
		FallbackSkin:      "default",
		UseColorsFromSkin: false,
		UseBeatmapColors:  false,
		Cursor: &skinCursor{
			UseSkinCursor:    false,
			Scale:            1.0,
			TrailScale:       1.0,
			ForceLongTrail:   false,
			LongTrailLength:  2048,
			LongTrailDensity: 1.0,
		},
	}
}

type skin struct {
	CurrentSkin       string `combo:"true" comboSrc:"SkinOptions" search:"true" liveedit:"false"`
	FallbackSkin      string `combo:"true" comboSrc:"SkinOptions" search:"true" liveedit:"false"`
	UseColorsFromSkin bool
	UseBeatmapColors  bool

	Cursor *skinCursor
}

var skinPath string
var skinLazerMode bool
var skinLazerPath string
var skinCache []string

// LazerSkinLister returns custom skin names from osu!lazer when LazerMode is active.
var LazerSkinLister func() []string

func (d *defaultsFactory) SkinOptions() []string {
	curSkinsDir := General.GetSkinsDir()
	curLazerMode := General.LazerMode
	curLazerPath := ""
	if curLazerMode {
		curLazerPath = General.GetLazerPath()
	}

	if curSkinsDir != skinPath || curLazerMode != skinLazerMode || curLazerPath != skinLazerPath || skinCache == nil {
		skinPath = curSkinsDir
		skinLazerMode = curLazerMode
		skinLazerPath = curLazerPath

		skinCache = []string{}
		seen := map[string]bool{
			"default": true,
		}

		fs, err := ioutil.ReadDir(skinPath)
		if err == nil {
			for _, f := range fs {
				if f.IsDir() {
					name := filepath.Base(f.Name())
					lower := strings.ToLower(name)
					if !seen[lower] {
						seen[lower] = true
						skinCache = append(skinCache, name)
					}
				}
			}
		}

		if curLazerMode && LazerSkinLister != nil {
			for _, name := range LazerSkinLister() {
				lower := strings.ToLower(name)
				if name != "" && !seen[lower] {
					seen[lower] = true
					skinCache = append(skinCache, name)
				}
			}
		}

		sort.Slice(skinCache, func(i, j int) bool {
			return strings.ToLower(skinCache[i]) < strings.ToLower(skinCache[j])
		})

		skinCache = append([]string{"default"}, skinCache...)
	}

	return skinCache
}

type skinCursor struct {
	UseSkinCursor    bool
	Scale            float64 `max:"2"`
	TrailScale       float64 `max:"2"`
	ForceLongTrail   bool
	LongTrailLength  int64   `max:"10000"`
	LongTrailDensity float64 `min:"0.1" max:"3"`
}
