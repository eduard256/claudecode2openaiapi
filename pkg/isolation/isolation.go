package isolation

import (
	"errors"
	"os"
	"path/filepath"
)

// Hardcoded paths. Override via env CC2OA_DATA_DIR if you really must.
const (
	defaultDataDir = "/var/lib/claudecode2openaiapi"
)

var (
	DataDir    string
	FakeHome   string
	WorkDir    string
	TokensFile string
)

func init() {
	DataDir = os.Getenv("CC2OA_DATA_DIR")
	if DataDir == "" {
		DataDir = defaultDataDir
	}
	FakeHome = filepath.Join(DataDir, "fakehome")
	WorkDir = filepath.Join(DataDir, "workspace")
	TokensFile = filepath.Join(DataDir, "tokens.json")
}

// Setup creates the isolated environment for spawning claude:
//   - DataDir/fakehome/.claude/.credentials.json -> $REAL_HOME/.claude/.credentials.json
//   - DataDir/workspace as a clean cwd
//
// Without this, claude would auto-discover CLAUDE.md and write project memory
// to the real ~/.claude. With this, claude sees an empty home and a clean cwd.
func Setup() error {
	if err := os.MkdirAll(WorkDir, 0o755); err != nil {
		return err
	}

	claudeDir := filepath.Join(FakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return err
	}

	realHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	src := filepath.Join(realHome, ".claude", ".credentials.json")
	dst := filepath.Join(claudeDir, ".credentials.json")

	if _, err := os.Lstat(src); err != nil {
		return errors.New("isolation: real ~/.claude/.credentials.json not found, run `claude /login` first")
	}

	if _, err := os.Lstat(dst); err == nil {
		return nil
	}
	return os.Symlink(src, dst)
}
