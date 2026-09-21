package reviewer

import (
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(`^@@ -[0-9]+(?:,([0-9]+))? \+[0-9]+(?:,([0-9]+))? @@`)

// DiffFiles extracts repository-relative names from supported diff metadata.
// It does not validate the diff or map comment locations to hunks.
func DiffFiles(diff string) map[string]struct{} {
	files := make(map[string]struct{})
	add := func(path string) {
		if path != "/dev/null" && ValidPath(path) {
			files[path] = struct{}{}
		}
	}
	lines := strings.Split(diff, "\n")
	gitSection, inHunk, binaryBody := false, false, false
	oldLeft, newLeft := uint64(0), uint64(0)
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "diff --git ") {
			gitSection, inHunk, binaryBody = true, false, false
			oldName, newName, ok := gitNames(strings.TrimPrefix(line, "diff --git "))
			if ok {
				add(oldName)
				add(newName)
			}
			continue
		}
		if strings.HasPrefix(line, "diff ") {
			gitSection, inHunk, binaryBody = false, false, true
			continue
		}
		if binaryBody {
			continue
		}
		if inHunk {
			if line == `\ No newline at end of file` {
				continue
			}
			if len(line) > 0 {
				switch line[0] {
				case ' ':
					decrement(&oldLeft)
					decrement(&newLeft)
				case '-':
					decrement(&oldLeft)
				case '+':
					decrement(&newLeft)
				default:
					continue
				}
			}
			inHunk = oldLeft != 0 || newLeft != 0
			continue
		}
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			oldLeft, newLeft = ^uint64(0), ^uint64(0)
			if match := hunkHeader.FindStringSubmatch(line); match != nil {
				oldLeft, newLeft = hunkCount(match[1]), hunkCount(match[2])
				inHunk = oldLeft != 0 || newLeft != 0
			}
			continue
		}
		if line == "GIT binary patch" {
			binaryBody = true
			continue
		}
		if strings.HasPrefix(line, "--- ") && index+1 < len(lines) && strings.HasPrefix(lines[index+1], "+++ ") {
			oldName, oldOK := headerName(line[4:], "a/")
			newName, newOK := headerName(lines[index+1][4:], "b/")
			if oldOK && newOK {
				add(oldName)
				add(newName)
			}
			index++
			continue
		}
		if gitSection {
			for _, prefix := range []string{"rename from ", "rename to ", "copy from ", "copy to "} {
				if strings.HasPrefix(line, prefix) {
					if name, ok := pathName(strings.TrimPrefix(line, prefix)); ok {
						add(name)
					}
					break
				}
			}
		}
	}
	return files
}

func decrement(value *uint64) {
	if *value > 0 {
		*value--
	}
}

func hunkCount(raw string) uint64 {
	if raw == "" {
		return 1
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return ^uint64(0)
	}
	return value
}

func pathName(raw string) (string, bool) {
	if strings.HasPrefix(raw, `"`) {
		name, err := strconv.Unquote(raw)
		return name, err == nil
	}
	return raw, raw != ""
}

func headerName(raw, prefix string) (string, bool) {
	raw = strings.SplitN(raw, "\t", 2)[0]
	name, ok := pathName(raw)
	if !ok {
		return "", false
	}
	return strings.TrimPrefix(name, prefix), true
}

func gitNames(raw string) (string, string, bool) {
	var first, second string
	if strings.HasPrefix(raw, `"`) {
		end := quotedEnd(raw)
		if end < 0 || end+1 >= len(raw) || raw[end+1] != ' ' {
			return "", "", false
		}
		first, second = raw[:end+1], raw[end+2:]
	} else {
		positions := []int{}
		for index := range raw {
			if strings.HasPrefix(raw[index:], " b/") || strings.HasPrefix(raw[index:], ` "b/`) {
				positions = append(positions, index)
			}
		}
		if len(positions) != 1 {
			return "", "", false
		}
		first, second = raw[:positions[0]], raw[positions[0]+1:]
	}
	oldName, oldOK := pathName(first)
	newName, newOK := pathName(second)
	if !oldOK || !newOK || !strings.HasPrefix(oldName, "a/") || !strings.HasPrefix(newName, "b/") {
		return "", "", false
	}
	return strings.TrimPrefix(oldName, "a/"), strings.TrimPrefix(newName, "b/"), true
}

func quotedEnd(value string) int {
	for index := 1; index < len(value); index++ {
		if value[index] == '\\' {
			index++
			continue
		}
		if value[index] == '"' {
			return index
		}
	}
	return -1
}
