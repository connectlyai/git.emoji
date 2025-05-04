package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var allTypes []*Type
var mapTypes map[string]*Type

func main() {
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}

	switch arg {
	case "", "emoji", "help", "-h", "--help":
		printHelp()
		os.Exit(0)
	}

	// if not in git repo, run git command directly
	if !_tryInit() {
		execGit(os.Args[1:])
		return
	}

	switch arg {
	case "write-config":
		debugf("git.emoji %q", os.Args[1:])
		loadConfig()
		writeConfigFile(allTypes)

	case "rev-parse":
		// no setup hooks
		execGit(os.Args[1:])

	case gmoji(_prepareCommitMsg):
		debugf("git.emoji %q", os.Args[1:])
		loadConfig()
		execPrepareCommitMsg(os.Args[2:])

	case gmoji(_commitMsg):
		debugf("git.emoji %q", os.Args[1:])
		loadConfig()
		execCommitMsg(os.Args[2:])

	case "setup-hooks":
		debugf("git.emoji %q", os.Args[1:])
		setupHooks()
		infof("✅ Successfully setup git hooks")

	case "remove-hooks":
		debugf("git.emoji %q", os.Args[1:])
		removeHooks()
		infof("✅ Successfully removed git hooks")

	case "commit":
		if !isOptOut() {
			setupHooks()
			loadConfig()
			execCommit(os.Args[1:])
			return
		}
		fallthrough

	default:
		setupHooks()
		execGit(os.Args[1:])
	}
}

func which(command string, env []string) string {
	stdout := &strings.Builder{}
	cmd := exec.Command("which", command)
	cmd.Env = env
	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
}

func execGit(args []string) {
	debugf("%v %q", origGit(), args)

	cmd := exec.Command(origGit(), args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err == nil {
		return
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exit(exitErr.ExitCode())
	}
	fatalf("%v", err)
}

func execGitx(args ...string) (string, string, error) {
	debugf("%v %q", origGit(), args)

	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	cmd := exec.Command(origGit(), args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr

	err := cmd.Run()
	return strings.TrimSpace(stdout.String()), stderr.String(), err
}

func execCommit(args []string) {
	getFlagType := func() (_ *Type, remain []string) {
		for i, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				continue
			}
			arg = strings.TrimPrefix(arg, "-")
			arg = strings.TrimPrefix(arg, "-")
			if t, ok := mapTypes[arg]; ok {
				var _args []string
				_args = append(_args, args[:i]...)
				_args = append(_args, args[i+1:]...)
				return t, _args
			}
		}
		return nil, args
	}
	editMsgArg := func(emoji string) (_ []string, ok bool) {
		for i, arg := range args {
			if arg != "-m" && strings.HasPrefix(arg, "-m") {
				msg := strings.TrimPrefix(arg, "-m")
				args[i] = "-m" + emoji + " " + msg
				ok = true
				continue
			}
			if arg != "-m" {
				continue
			}
			if len(args) > i+1 {
				args[i+1] = emoji + " " + args[i+1]
				ok = true
			}
		}
		return args, ok
	}

	// skip prompt if there is no -m arg
	if !slices.Contains(args, "-m") {
		execGit(args)
		return
	}

	// detect -feat, -ch, etc. or show prompt if missing
	idx := 0
	flagType, args := getFlagType()
	if flagType == nil {
		flagType, idx = askFlagType("")
	}
	if flagType == nil {
		fatalf("Can not read emoji!")
		return
	}

	// update -m 'message' with emoji
	args, _ = editMsgArg(flagType.Icons[idx])
	execGit(args)
}

func askFlagType(firstLine string) (_ *Type, idx int) {
	reNum := regexp.MustCompile(`^\d+`)
	reTxt := regexp.MustCompile(`^[a-z]+`)
	parse := func(re *regexp.Regexp, s string) (string, string, bool) {
		first := re.FindString(s)
		return first, strings.TrimPrefix(s, first), first != ""
	}

	ft, ch := getType("Features"), getType("Chores")

	fmt.Println()
	fmt.Println("--- 👉 Please choose an emoji 👈 ----------------------")
	fmt.Println()
	printHelpEmojis(os.Stdout, "")
	fmt.Printf("\nHINT: You can use command line flag to choose the type:\n")
	fmt.Printf("      git commit -feat -m 'message'   # %s Features\n", ft.Icons[0])
	fmt.Printf("      git commit -ft   -m 'message'   # %s Features\n", ft.Icons[0])
	fmt.Printf("      git commit -ft1  -m 'message'   # %s Features\n", ft.Icons[1])
	fmt.Printf("      git commit -ch   -m 'message'   # %s Chore\n", ch.Icons[0])
	fmt.Printf("      git commit -ch1  -m 'message'   # %s Chore\n", ch.Icons[1])
	fmt.Println("")
	if firstLine != "" {
		fmt.Println("--- 👉 Your commit message 👈 -------------------------")
		fmt.Printf("%s\n\n", firstLine)
	}

	input, prompt := "", "Enter a number or abbr or emoji (1 | 1a | ft | ft1): "
	mapEmoji := mapEmojis()
	for {
		fmt.Printf("\r%s\r", strings.Repeat(" ", len(prompt)+len(input)+1))
		fmt.Print(prompt)

		input = readLine()
		in := strings.Trim(input, "- \t\n")

		if first, second, ok := parse(reNum, in); ok {
			id := must(strconv.Atoi(first))
			id--
			if id < 0 || id >= len(allTypes) {
				continue
			}
			typ := allTypes[id]
			if second == "" {
				return typ, 0
			}
			idx = int(second[0]-'a') + 1
			if idx < 0 || idx >= len(typ.Icons) {
				continue
			}
			return typ, idx
		}
		if first, second, ok := parse(reTxt, in); ok {
			typ := mapTypes[first]
			if typ == nil {
				continue
			}
			if second == "" {
				return typ, 0
			}
			var err error
			idx, err = strconv.Atoi(second)
			if err != nil {
				continue
			}
			if idx < 0 || idx >= len(typ.Icons) {
				continue
			}
			return typ, idx
		}
		if _, ok := mapEmoji[in]; ok {
			return &Type{Icons: []string{in}}, 0
		}
	}
}

func printHelp() {
	msg := `git.emoji - git commit with emoji

git.emoji is a tool to commit your changes with emoji. It provides a simple command-line interface to help you select the right emoji for your commit. It will setup git hooks to commit with emoji, and can be used as a wrapper of git.

SETUP:
  git.emoji setup-hooks

USAGE:
  git.emoji commit -feat -m 'message'   # Features
  git.emoji commit -ft   -m 'message'   # Features
  git.emoji commit -ft1  -m 'message'   # Features
  git.emoji commit -ch   -m 'message'   # Chore
  git.emoji commit -ch1  -m 'message'   # Chore

OPTIONAL: add this to your .zshrc or .bashrc:

  alias git=git.emoji

then use git.emoji as git alias:
  git commit -feat -m 'message'   # Features
  git commit -ft   -m 'message'   # Features
  git commit -ft1  -m 'message'   # Features
  git commit -ch   -m 'message'   # Chore
  git commit -ch1  -m 'message'   # Chore

CONFIG: run this command to customize your emoji:

  git.emoji write-config
`
	gitHelp, _, _ := execGitx("--help")
	gitHelp = strings.TrimSpace(gitHelp)
	if gitHelp != "" {
		msg += "\n\n⎯⎯⎯\n\n" + gitHelp
	}

	printf(os.Stderr, "%s", msg)
}

func printHelpEmojis(w io.Writer, prefix string) {
	pr := func(format string, args ...any) {
		must(fmt.Fprintf(w, format, args...))
	}

	for i, typ := range allTypes {
		pr(prefix)
		pr("% 3d. % 16s\t", i+1, typ.Name)
		for _, icon := range typ.Icons {
			pr("%s ", icon)
		}
		pr("\t ")
		for _, alias := range typ.Alias {
			pr("-%s ", alias)
		}
		pr("\n")
	}
}
