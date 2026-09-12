package reviewer

import (
	"regexp"
	"strconv"
	"strings"
)

var hunkHeader = regexp.MustCompile(`^@@ -[0-9]+(?:,([0-9]+))? \+[0-9]+(?:,([0-9]+))? @@`)

// DiffFiles extracts names from conventional Git and unified diff metadata. It
// deliberately does not validate the diff or map line positions to hunks.
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
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "diff --git ") {
			gitSection, inHunk, binaryBody = true, false, false
			a, b, ok := gitNames(strings.TrimPrefix(line, "diff --git "))
			if ok {
				add(a)
				add(b)
			}
			continue
		}
		if strings.HasPrefix(line, "diff ") {
			gitSection = false
			inHunk = false
			binaryBody = true
			continue
		}
		if binaryBody {
			continue
		}
		if inHunk {
			if line == "\\ No newline at end of file" {
				continue
			}
			if len(line) > 0 {
				switch line[0] {
				case ' ':
					if oldLeft > 0 {
						oldLeft--
					}
					if newLeft > 0 {
						newLeft--
					}
				case '-':
					if oldLeft > 0 {
						oldLeft--
					}
				case '+':
					if newLeft > 0 {
						newLeft--
					}
				default:
					continue
				}
			}
			if oldLeft == 0 && newLeft == 0 {
				inHunk = false
			}
			continue
		}
		if strings.HasPrefix(line, "@@") {
			// Unrecognized hunks consume the remainder until the next Git section.
			inHunk = true
			oldLeft, newLeft = ^uint64(0), ^uint64(0)
			if match := hunkHeader.FindStringSubmatch(line); match != nil {
				count := func(s string) uint64 {
					if s == "" {
						return 1
					}
					n, err := strconv.ParseUint(s, 10, 64)
					if err != nil {
						return ^uint64(0)
					}
					return n
				}
				oldLeft, newLeft = count(match[1]), count(match[2])
				inHunk = oldLeft != 0 || newLeft != 0
			}
			continue
		}
		if line == "GIT binary patch" {
			binaryBody = true
			continue
		}
		if strings.HasPrefix(line, "--- ") && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "+++ ") {
			a, aok := headerName(line[4:], "a/")
			b, bok := headerName(lines[i+1][4:], "b/")
			if aok && bok {
				add(a)
				add(b)
			}
			i++
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
	// Quoted Git paths are unambiguous; unquoted space-containing paths use
	// the conventional destination prefix as a delimiter only when unique.
	var first, second string
	if strings.HasPrefix(raw, `"`) {
		end := quotedEnd(raw)
		if end < 0 || end+1 >= len(raw) || raw[end+1] != ' ' {
			return "", "", false
		}
		first, second = raw[:end+1], raw[end+2:]
	} else {
		positions := []int{}
		for i := 0; i < len(raw); i++ {
			if strings.HasPrefix(raw[i:], " b/") || strings.HasPrefix(raw[i:], ` "b/`) {
				positions = append(positions, i)
			}
		}
		if len(positions) != 1 {
			return "", "", false
		}
		first, second = raw[:positions[0]], raw[positions[0]+1:]
	}
	a, aok := pathName(first)
	b, bok := pathName(second)
	if !aok || !bok || !strings.HasPrefix(a, "a/") || !strings.HasPrefix(b, "b/") {
		return "", "", false
	}
	return strings.TrimPrefix(a, "a/"), strings.TrimPrefix(b, "b/"), true
}
func quotedEnd(s string) int {
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '"' {
			return i
		}
	}
	return -1
}
