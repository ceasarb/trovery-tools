package session

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// IsGitRepo checks if the given path is inside a git repository.
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// Snapshot captures the current git state.
func Snapshot(projectRoot string) (*GitSnapshot, error) {
	if !IsGitRepo(projectRoot) {
		return nil, nil
	}

	headSHA, err := gitOutput(projectRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("getting HEAD: %w", err)
	}

	branch, err := gitOutput(projectRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		branch = "HEAD"
	}

	// Get dirty files (modified + untracked).
	dirty, err := gitOutputLines(projectRoot, "status", "--porcelain", "--no-renames")
	if err != nil {
		dirty = nil
	}

	var dirtyFiles []string
	for _, line := range dirty {
		if len(line) > 3 {
			dirtyFiles = append(dirtyFiles, strings.TrimSpace(line[3:]))
		}
	}

	return &GitSnapshot{
		HeadSHA:    headSHA,
		Branch:     branch,
		DirtyFiles: dirtyFiles,
	}, nil
}

// DiffSince computes file changes since the given snapshot,
// including committed, staged, unstaged, and untracked changes.
func DiffSince(projectRoot string, snapshot *GitSnapshot) ([]FileChange, error) {
	if snapshot == nil {
		return nil, nil
	}

	// Committed changes (start SHA to HEAD).
	committed, err := DiffBetweenRefs(projectRoot, snapshot.HeadSHA, "HEAD")
	if err != nil {
		return nil, err
	}

	// Staged changes (index vs HEAD).
	staged, err := diffWorkingTree(projectRoot, true)
	if err != nil {
		return nil, err
	}

	// Unstaged changes (working tree vs index).
	unstaged, err := diffWorkingTree(projectRoot, false)
	if err != nil {
		return nil, err
	}

	// Untracked files.
	untracked, err := untrackedFiles(projectRoot)
	if err != nil {
		return nil, err
	}

	// Merge all changes, deduplicating by path.
	// Priority: committed > staged > unstaged > untracked.
	seen := make(map[string]bool)
	var changes []FileChange

	for _, c := range committed {
		seen[c.Path] = true
		changes = append(changes, c)
	}
	for _, c := range staged {
		if !seen[c.Path] {
			seen[c.Path] = true
			changes = append(changes, c)
		}
	}
	for _, c := range unstaged {
		if !seen[c.Path] {
			seen[c.Path] = true
			changes = append(changes, c)
		}
	}
	for _, c := range untracked {
		if !seen[c.Path] {
			seen[c.Path] = true
			changes = append(changes, c)
		}
	}

	return changes, nil
}

// DiffBetweenRefs computes committed file changes between two git refs.
func DiffBetweenRefs(projectRoot, baseRef, headRef string) ([]FileChange, error) {
	lines, err := gitOutputLines(projectRoot, "diff", "--numstat", baseRef+"..."+headRef)
	if err != nil {
		// Fallback: try without merge-base (three dots → two dots).
		lines, err = gitOutputLines(projectRoot, "diff", "--numstat", baseRef+".."+headRef)
		if err != nil {
			return nil, fmt.Errorf("computing diff: %w", err)
		}
	}

	var changes []FileChange
	for _, line := range lines {
		if line == "" {
			continue
		}
		change, err := parseNumstatLine(line)
		if err != nil {
			continue
		}
		change.Source = "committed"
		changes = append(changes, change)
	}

	return changes, nil
}

// diffWorkingTree captures staged or unstaged changes in the working tree.
func diffWorkingTree(projectRoot string, staged bool) ([]FileChange, error) {
	args := []string{"diff", "--numstat"}
	source := "unstaged"
	if staged {
		args = append(args, "--cached")
		source = "staged"
	}

	lines, err := gitOutputLines(projectRoot, args...)
	if err != nil {
		return nil, fmt.Errorf("computing %s diff: %w", source, err)
	}

	var changes []FileChange
	for _, line := range lines {
		if line == "" {
			continue
		}
		change, err := parseNumstatLine(line)
		if err != nil {
			continue
		}
		change.Source = source
		changes = append(changes, change)
	}

	return changes, nil
}

// untrackedFiles returns untracked files as FileChange entries.
func untrackedFiles(projectRoot string) ([]FileChange, error) {
	files, err := gitOutputLines(projectRoot, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	var changes []FileChange
	for _, f := range files {
		if f == "" {
			continue
		}
		changes = append(changes, FileChange{
			Path:       f,
			ChangeType: "added",
			Source:     "untracked",
		})
	}

	return changes, nil
}

// ReadFileAtRef reads a file's content at a specific git ref.
func ReadFileAtRef(projectRoot, filePath, ref string) (string, error) {
	content, err := gitOutput(projectRoot, "show", ref+":"+filePath)
	if err != nil {
		return "", fmt.Errorf("reading %s at %s: %w", filePath, ref, err)
	}
	return content, nil
}

func parseNumstatLine(line string) (FileChange, error) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return FileChange{}, fmt.Errorf("invalid numstat line: %s", line)
	}

	additions, _ := strconv.Atoi(parts[0])
	deletions, _ := strconv.Atoi(parts[1])
	path := parts[2]

	// Handle renames: "old => new".
	if strings.Contains(path, " => ") {
		path = strings.Split(path, " => ")[1]
		path = strings.Trim(path, "{}") // Handle {old => new} format.
	}

	changeType := "modified"
	if additions > 0 && deletions == 0 {
		// Could be added, but numstat doesn't distinguish — check if binary.
		if parts[0] == "-" {
			changeType = "modified" // binary
		}
	}

	return FileChange{
		Path:       filepath.Clean(path),
		ChangeType: changeType,
		Additions:  additions,
		Deletions:  deletions,
	}, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitOutputLines(dir string, args ...string) ([]string, error) {
	out, err := gitOutput(dir, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
