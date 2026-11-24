package files

import (
	"os"
	"testing"
)

func TestVirtualFileMap(t *testing.T) {
	entries := map[string]string{
		`Audio\Lost_Dedicated.mp3`: `C:\osu\files\1\14\hash_audio`,
		`BG.jpg`:                   `C:\osu\files\0\0e\hash_bg`,
		`soft-hitnormal2.wav`:      `C:\osu\files\a\ab\hash_hs`,
	}

	fm := NewVirtualFileMap(entries)

	tests := []struct {
		query string
		want  string
	}{
		{"audio/lost_dedicated.mp3", `C:\osu\files\1\14\hash_audio`},
		{`Audio\Lost_Dedicated.mp3`, `C:\osu\files\1\14\hash_audio`},
		{"/bg.jpg", `C:\osu\files\0\0e\hash_bg`},
		{"SOFT-HITNORMAL2.WAV", `C:\osu\files\a\ab\hash_hs`},
	}

	for _, tc := range tests {
		got, err := fm.GetFile(tc.query)
		if err != nil {
			t.Fatalf("GetFile(%q) unexpected error: %v", tc.query, err)
		}
		if got != tc.want {
			t.Fatalf("GetFile(%q) = %q, want %q", tc.query, got, tc.want)
		}
	}

	if _, err := fm.GetFile("missing.png"); !os.IsNotExist(err) {
		t.Fatalf("GetFile(missing.png) error = %v, want os.ErrNotExist", err)
	}

	gotMap := fm.GetMap()
	if len(gotMap) != 3 {
		t.Fatalf("GetMap() len = %d, want 3", len(gotMap))
	}
	if gotMap["soft-hitnormal2.wav"] != `C:\osu\files\a\ab\hash_hs` {
		t.Fatalf("GetMap()[soft-hitnormal2.wav] = %q", gotMap["soft-hitnormal2.wav"])
	}
}
