package telegram

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseReviewTarget parses /review arguments into owner/repo and PR number.
// Forms: "142", "owner/repo#142", "owner/repo 142".
func ParseReviewTarget(arg string, defaultRepo string) (repo string, pr int, err error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", 0, fmt.Errorf("usage: /review <number> or /review owner/repo#<number>")
	}
	if i := strings.Index(arg, "#"); i >= 0 {
		repo = strings.TrimSpace(arg[:i])
		num := strings.TrimSpace(arg[i+1:])
		pr, err = strconv.Atoi(num)
		if err != nil || pr <= 0 {
			return "", 0, fmt.Errorf("invalid PR number after #")
		}
		if repo == "" {
			return "", 0, fmt.Errorf("missing owner/repo before #")
		}
		return repo, pr, nil
	}
	fields := strings.Fields(arg)
	if len(fields) == 2 && strings.Contains(fields[0], "/") {
		pr, err = strconv.Atoi(fields[1])
		if err != nil || pr <= 0 {
			return "", 0, fmt.Errorf("invalid PR number")
		}
		return strings.TrimSpace(fields[0]), pr, nil
	}
	if len(fields) == 1 {
		pr, err = strconv.Atoi(fields[0])
		if err != nil || pr <= 0 {
			return "", 0, fmt.Errorf("invalid PR number; use /review owner/repo#%s if default repo is unset", fields[0])
		}
		defaultRepo = strings.TrimSpace(defaultRepo)
		if defaultRepo == "" {
			return "", 0, fmt.Errorf("set ORB_REPO_NAME or use owner/repo#%d", pr)
		}
		return defaultRepo, pr, nil
	}
	return "", 0, fmt.Errorf("could not parse review target")
}

// ParseRepoArg returns a single owner/repo from /watch or /unwatch arguments.
func ParseRepoArg(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", fmt.Errorf("usage: /watch owner/repo")
	}
	if strings.Contains(arg, " ") {
		return "", fmt.Errorf("use a single owner/repo identifier")
	}
	return arg, nil
}

// SplitMessageChunks splits text into Telegram-safe chunks (max ~4000 runes per message).
func SplitMessageChunks(s string, maxRunes int) []string {
	if maxRunes <= 0 {
		maxRunes = 4000
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		if len(s) == 0 {
			return nil
		}
		return []string{s}
	}
	var out []string
	for len(runes) > 0 {
		n := maxRunes
		if n > len(runes) {
			n = len(runes)
		}
		out = append(out, string(runes[:n]))
		runes = runes[n:]
	}
	return out
}
