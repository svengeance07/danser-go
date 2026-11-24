package files

import (
	"os"
	"path/filepath"
	"strings"
)

type FileMap struct {
	path      string
	pathCache map[string]string
	virtual   bool
}

func NewVirtualFileMap(entries map[string]string) *FileMap {
	fileMap := &FileMap{
		pathCache: make(map[string]string, len(entries)),
		virtual:   true,
	}

	for name, physicalPath := range entries {
		norm := strings.TrimPrefix(strings.ReplaceAll(strings.ToLower(name), "\\", "/"), "/")
		fileMap.pathCache[norm] = physicalPath
	}

	return fileMap
}

func NewFileMap(path string) (*FileMap, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}

	fPath := strings.ReplaceAll(path, "\\", "/")
	if !strings.HasSuffix(fPath, "/") {
		fPath += "/"
	}

	fileMap := &FileMap{
		path:      fPath,
		pathCache: make(map[string]string),
	}

	results, _ := SearchFiles(fPath, "*", -1)

	for _, result := range results {
		fixedPath := strings.TrimPrefix(strings.ReplaceAll(result, "\\", "/"), fPath)
		fileMap.pathCache[strings.ToLower(fixedPath)] = fixedPath
	}

	return fileMap, nil
}

func (f *FileMap) GetFile(path string) (string, error) {
	if f.virtual {
		fPath := strings.TrimPrefix(strings.ReplaceAll(strings.ToLower(path), "\\", "/"), "/")
		if resolved, ok := f.pathCache[fPath]; ok {
			return resolved, nil
		}

		return "", os.ErrNotExist
	}

	sPath := strings.ToLower(f.path)
	fPath := strings.TrimPrefix(strings.ReplaceAll(strings.ToLower(path), "\\", "/"), sPath)

	if resolved, ok := f.pathCache[fPath]; ok {
		return filepath.Join(f.path, resolved), nil
	}

	return "", os.ErrNotExist
}

func (f *FileMap) GetMap() map[string]string {
	retMap := make(map[string]string, len(f.pathCache))

	for k, v := range f.pathCache {
		if f.virtual {
			retMap[k] = v
		} else {
			retMap[k] = filepath.Join(f.path, v)
		}
	}

	return retMap
}
