package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var isDebug = os.Getenv("GIT_EMOJI_DEBUG") == "1"

var _origGit string
var _emojiGit string
var _rootRepoDir string // the top level directory of the project
var _rootGitDir string  // the .git directory of the project
var _isRootRepo bool    // is it the root repository

func origGit() string {
	if _origGit != "" {
		return _origGit
	}
	_origGit = findOriginalGit(emojiGit())
	return _origGit
}
func emojiGit() string {
	if _emojiGit != "" {
		return _emojiGit
	}
	_emojiGit = findEmojiGit()
	return _emojiGit
}
func gmoji(x string) string { return "gmoji-" + x }
func gitDir() string {
	_init()
	return _rootGitDir
}
func rootRepoDir() string {
	_init()
	return _rootRepoDir
}

func _init() {
	if _rootRepoDir != "" {
		return
	}
	gmust := func(stderr string, err error, msg string, args ...any) {
		if err != nil {
			must(fmt.Fprint(os.Stderr, stderr))
			printf(os.Stderr, "%v", err)
			fatalf(msg, args...)
		}
	}

	gtop, gerr, err := execGitx("rev-parse", "--show-toplevel")
	gmust(gerr, err, "not a git directory")

	gdir, gerr, err := execGitx("rev-parse", "--git-dir")
	gmust(gerr, err, "not a git directory")

	debugf("git dir %q", gdir)
	gtop = must(filepath.Abs(gtop))
	gdir = must(filepath.Abs(gdir))

	_isRootRepo = gdir == gtop+"/.git"
	if _isRootRepo {
		_rootRepoDir = gtop
		_rootGitDir = gdir
	} else {
		idx := strings.LastIndex(gdir, ".git")
		if idx <= 0 {
			fatalf("not a git directory (can not find the repository root dir)")
		}
		_rootRepoDir = gdir[:idx]
		_rootGitDir = filepath.Join(_rootRepoDir, ".git")
	}
}

func findEmojiGit() string {
	arg0 := os.Args[0]
	if strings.Contains(arg0, "/") {
		return must(filepath.Abs(arg0))
	}
	path := which(arg0, os.Environ())
	if path == "" {
		fatalf("can not find %q in PATH", arg0)
	}
	return path
}

func findOriginalGit(emojiGit string) string {
	debugf("finding original git")

	env := slices.Clone(os.Environ())
	whichGit := func(path string) string {
		for i, e := range env {
			if strings.HasPrefix(e, "PATH=") {
				env[i] = "PATH=" + path
				break
			}
		}
		return which("git", env)
	}

	path := os.Getenv("PATH")
	for {
		parts := strings.SplitN(path, ":", 2)
		if len(parts) <= 1 {
			fatalf("can not find original git")
		}
		path = parts[1]
		orgGit := whichGit(path)
		if orgGit != emojiGit {
			return orgGit
		}
	}
}

func readLine() string {
	var buf strings.Builder
	for {
		var data [1]byte
		must(os.Stdin.Read(data[:]))
		if data[0] == '\n' {
			break
		}
		buf.WriteByte(data[0])
	}
	return buf.String()
}

func isOptOut() bool {
	path := rootRepoDir() + "/.git/emoji.not"
	_, err := os.Stat(path)
	if err == nil {
		debugf(".git/emoji.not exists, use original git")
	}
	return err == nil
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
func exit(code int) {
	debugf("exit status %d", code)
	os.Exit(code)
}
func debugf(format string, args ...any) {
	if isDebug {
		must(fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...))
	}
}
func printf(w io.Writer, format string, args ...any) int {
	return must(fmt.Fprintf(w, format, args...))
}
func infof(format string, args ...any) {
	must(fmt.Fprintf(os.Stderr, format+"\n", args...))
}
func errorf(format string, args ...any) {
	must(fmt.Fprintf(os.Stderr, "\nERROR: "+format+"\n", args...))
}
func fatalf(format string, args ...any) {
	errorf(format, args...)
	exit(123)
}
