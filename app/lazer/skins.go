package lazer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wieku/danser-go/app/settings"
	"github.com/wieku/danser-go/app/skin"
	"github.com/wieku/danser-go/framework/files"
)

// LazerSkinInfo represents a skin entry returned by `lazer-bridge list-skins`.
type LazerSkinInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Creator   string `json:"creator"`
	FileCount int    `json:"fileCount"`
}

// ResolvedSkin represents the JSON response from `lazer-bridge resolve-skin`.
type ResolvedSkin struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Creator string            `json:"creator"`
	Files   map[string]string `json:"files"`
}

func init() {
	settings.LazerSkinLister = func() []string {
		skins, err := ListSkins()
		if err != nil {
			return nil
		}

		names := make([]string, 0, len(skins))
		for _, s := range skins {
			names = append(names, s.Name)
		}
		return names
	}

	skin.LazerSkinLoader = LoadSkinFileMap
}

// ListSkins queries lazer-bridge for custom skins stored in osu!lazer's client.realm.
func ListSkins() ([]LazerSkinInfo, error) {
	out, err := runBridge("list-skins")
	if err != nil {
		return nil, err
	}

	var skins []LazerSkinInfo
	if err := json.Unmarshal(out, &skins); err != nil {
		return nil, fmt.Errorf("failed to decode list-skins output: %w", err)
	}

	return skins, nil
}

// ResolveSkinByName queries lazer-bridge for all virtual files belonging to a skin.
func ResolveSkinByName(name string) (*ResolvedSkin, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("empty skin name")
	}

	out, err := runBridge("resolve-skin", "--name", name)
	if err != nil {
		return nil, err
	}

	var resolved ResolvedSkin
	if err := json.Unmarshal(out, &resolved); err != nil {
		return nil, fmt.Errorf("failed to decode resolve-skin output: %w", err)
	}

	return &resolved, nil
}

// LoadSkinFileMap builds a virtual FileMap for an osu!lazer skin by name.
func LoadSkinFileMap(name string) (*files.FileMap, error) {
	resolved, err := ResolveSkinByName(name)
	if err != nil {
		return nil, err
	}

	if len(resolved.Files) == 0 {
		return nil, fmt.Errorf("osu!lazer skin %q has no files", name)
	}

	return files.NewVirtualFileMap(resolved.Files), nil
}
