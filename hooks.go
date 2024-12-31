package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	gmojiStartMark = "## START GIT.EMOJI"
	gmojiEndMark   = "## END GIT.EMOJI"

	_remove           = "remove"
	_commitMsg        = "commit-msg"
	_prepareCommitMsg = "prepare-commit-msg"
)

const initCommitMsg = "#!/bin/bash\n"
const initPrepareCommitMsg = `#!/bin/bash

COMMIT_MSG_FILE=$1
COMMIT_SOURCE=$2
SHA1=$3

`

func hookContent(hook string) string {
	return fmt.Sprintf(`%s
if [[ -x %s ]]; then
    %s gmoji-%s "$@" < /dev/tty || exit $?
fi
%s`, gmojiStartMark, emojiGit(), emojiGit(), hook, gmojiEndMark)
}

func setupHooks() bool {
	debugf("setup hooks")
	st, err := os.Stat(gitDir())
	if err != nil {
		debugf("not a git repository (ignored, todo)")
		return false
	}
	if !st.IsDir() {
		debugf(".git is not a directory (ignored, todo)")
		return false
	}

	setupHook(_commitMsg, initCommitMsg)
	setupHook(_prepareCommitMsg, initPrepareCommitMsg)
	return true
}

func removeHooks() {
	setupHook(_commitMsg, _remove)
	setupHook(_prepareCommitMsg, _remove)
}

func setupHook(hook, initContent string) {
	isSetup := initContent != _remove
	isRemove := initContent == _remove

	// read file
	filePath := filepath.Join(gitDir(), "hooks", hook)
	debugf("hook: %v", filePath)
	data, err := os.ReadFile(filePath)
	switch {
	case err == nil:
		break
	case os.IsNotExist(err) && isRemove:
		return
	case os.IsNotExist(err) && isSetup:
		break
	default:
		fatalf("reading %v: %v", hook, err)
	}

	// verify and initialize the content
	dataStr := strings.TrimSpace(string(data))
	if !strings.HasPrefix(dataStr, `#!`) {
		dataStr = initContent
	}

	// verify if the hook is already installed
	hookContentStr := hookContent(hook)
	if isSetup && strings.Contains(dataStr, hookContentStr) {
		debugf("hook %v already installed", hook)
		return
	}

	// remove old content
	dataStr = removeHook(hook, dataStr)

	// append new content
	if isSetup {
		dataStr += "\n" + hookContentStr + "\n"
	}

	// write back
	err = os.WriteFile(filePath, []byte(dataStr), 0755)
	if err != nil {
		fatalf("writing %v: %v", hook, err)
	}
	if isSetup {
		debugf("installed %v hook", hook)
	} else {
		debugf("removed %v hook", hook)
	}
}

func removeHook(hook, dataStr string) string {
	idx0 := strings.Index(dataStr, gmojiStartMark)
	idx1 := strings.Index(dataStr, gmojiEndMark)
	if idx0 >= 0 && idx1 >= 0 {
		if idx0 > idx1 {
			fatalf("invalid %v content (idx0 > idx1)", hook)
		}
		for idx0 >= 0 && dataStr[idx0-1] == '\n' {
			idx0--
		}
		dataStr = dataStr[:idx0] + dataStr[idx1+len(gmojiEndMark):]
	}
	return strings.TrimSpace(dataStr) + "\n"
}

func execCommitMsg(args []string) {
	if len(args) < 1 {
		fatalf("invalid commit-msg args: %v", args)
	}
	COMMIT_MSG_FILE := args[0]
	msgFile := commitMsgFile(COMMIT_MSG_FILE)
	dataStr := string(must(os.ReadFile(msgFile)))

	_, ok := validateMsgFile(dataStr)
	if !ok {
		fmt.Println("--------------------------------------------------")
		fmt.Println(strings.Split(dataStr, "\n")[0])
		fmt.Println("--------------------------------------------------")
		fatalf("commit message must start with an emoji")
	}
}

func execPrepareCommitMsg(args []string) {
	if len(args) < 1 {
		fatalf("invalid prepare-commit-msg args: %v", args)
	}
	COMMIT_MSG_FILE := args[0]
	msgFile := commitMsgFile(COMMIT_MSG_FILE)
	dataStr := string(must(os.ReadFile(msgFile)))

	firstLine, ok := validateMsgFile(dataStr)
	if ok {
		debugf("prepare commit message ok, skip")
		return
	}

	flagType, idx := askFlagType(firstLine)
	emoji := flagType.Icons[idx]
	debugf("emoji: %v", emoji)

	var b bytes.Buffer
	wroteExtra, wroteEmoji := false, false
	writeExtra := func() {
		if !wroteExtra {
			b.WriteString("#\n#\n")
			printHelpEmojis(&b, "# ")
			b.WriteString("#\n#\n")
		}
		wroteExtra = true
	}
	writeEmoji := func() {
		if !wroteEmoji {
			printf(&b, "%s ", emoji)
		}
		wroteEmoji = true
	}

	charN := printf(&b, "%s \n", emoji)
	lines := strings.Split(strings.TrimSpace(dataStr), "\n")
	for _, line := range lines {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		if strings.HasPrefix(line, "#") {
			writeExtra()
		} else {
			writeEmoji()
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	data := b.Bytes()
	if wroteEmoji {
		data = data[charN:]
	}

	err := os.WriteFile(msgFile, data, 0644)
	if err != nil {
		fatalf("preparing commit message: %v", err)
	}
	debugf("prepared commit message done")
}

func validateMsgFile(dataStr string) (firstLine string, ok bool) {
	lines := strings.Split(strings.TrimSpace(dataStr), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		firstLine = line
		break
	}

	for _, emoji := range allEmojis() {
		if strings.HasPrefix(firstLine, emoji) {
			return firstLine, true
		}
	}
	return firstLine, false
}

func commitMsgFile(commitMsgFile string) string {
	debugf("COMMIT_MSG_FILE: %v", commitMsgFile)
	if filepath.IsAbs(commitMsgFile) {
		return commitMsgFile
	}
	msgFile := must(filepath.Abs(commitMsgFile))
	if _, err := os.Stat(msgFile); err == nil {
		return msgFile
	}
	fatalf("COMMIT_MSG_FILE not found: %v", commitMsgFile)
	return ""
}
