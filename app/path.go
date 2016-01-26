package app

import "strings"

var mapping []PathInfo = make([]PathInfo, 0)

func AddPath(path PathInfo) {
	mapping = append(mapping, path)
}

func findLongestPrefix(mapping []PathInfo, path string) (info *PathInfo) {
	for _, iter := range mapping {
		if strings.HasPrefix(path, iter.Prefix) {
			if info == nil ||  len(info.Prefix) < len(iter.Prefix) {
				info = &iter
			}
		}
	}
	return
}