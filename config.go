package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Type struct {
	Name  string
	Alias []string
	Icons []string
}

func newType(name string) *Type {
	return &Type{Name: name}
}
func getType(name string) *Type {
	for _, typ := range allTypes {
		if typ.Name == name {
			return typ
		}
	}
	panic("unknown type: " + name)
}

func (t *Type) icon(icons ...string) *Type { t.Icons = append(t.Icons, icons...); return t }
func (t *Type) alias(as ...string) *Type   { t.Alias = append(t.Alias, as...); return t }

func defaultConfig() []*Type {
	return []*Type{
		newType("Features").icon("💻", "✨").alias("feat", "ft"),
		newType("Bug Fixes").icon("🚧", "🐛").alias("fix", "fx"),
		newType("SDKs/Libraries").icon("🛠️", "📦").alias("sdk", "lib", "pkg", "tenets"),
		newType("Breaking Changes").icon("🔥", "💥").alias("breaking", "br", "brk", "break"),
		newType("Code Refactoring").icon("♻️").alias("refactor", "rf", "ref", "rft"),
		newType("Infrastructure").icon("🐳").alias("infra", "if", "in", "inf"),
		newType("Tests").icon("🚨", "🧪").alias("test", "ts", "tst"),
		newType("Chores").icon("🧼", "🧹").alias("chore", "ch", "chr"),
		newType("Reverts").icon("⏳", "⏪").alias("revert", "rv", "rev", "rvt"),
		newType("Releases").icon("🚀", "🔖").alias("release", "rl", "rel", "rls"),
		newType("Others").icon("🔍").alias("other", "ot", "oth"),
	}
}

func defaultConfigFiles() []string {
	return []string{
		gitDir() + "/emoji.config",
		rootRepoDir() + "/emoji.config",
	}
}

// find config file in current repository or .git/emoji.config
func findConfigFile() (string, bool) {
	for _, file := range defaultConfigFiles() {
		if _, err := os.Stat(file); err == nil {
			return file, true
		}
	}
	return "", false
}

func loadConfig() {
	current, ok := findConfigFile()
	if ok {
		debugf("load config file %s\n", current)
		data, err := os.ReadFile(current)
		if err != nil {
			fatalf("failed to read config file %s: %s", current, err)
		}
		allTypes, err = parseConfig(data)
		if err != nil {
			fatalf("failed to parse config file %s: %s", current, err)
		}
	} else {
		debugf("no config file found")
		allTypes = defaultConfig()
	}
}

func writeConfigFile(config []*Type) {
	current, ok := findConfigFile()
	if ok {
		infof("Current config file: %s\n", current)
	} else {
		infof("No config file found.\n")
	}
	infof("Where do you want to write the config file?\n")
	for i, file := range defaultConfigFiles() {
		infof("  %d. %s\n", i+1, file)
	}
	infof("Enter a number: ")
	for {
		input := readLine()
		id, err := strconv.Atoi(input)
		if err != nil {
			continue
		}
		file := defaultConfigFiles()[id-1]
		must(0, os.WriteFile(file, marshalConfigFile(config), 0644))
		infof("config file written to %s\n", file)
		os.Exit(0)
	}
}

func marshalConfigFile(config []*Type) []byte {
	var buf bytes.Buffer
	for _, typ := range config {
		buf.WriteString(fmt.Sprintf("[git.emoji %q]\n", typ.Name))
		buf.WriteString("icons = ")
		buf.WriteString(strings.Join(typ.Icons, " "))
		buf.WriteString("\n")
		buf.WriteString("alias = ")
		buf.WriteString(strings.Join(typ.Alias, " "))
		buf.WriteString("\n")
	}
	return buf.Bytes()
}

// parse emoji.config
func parseConfig(data []byte) (out []*Type, outErr error) {
	var section *Type
	closeSection := func() {
		if section == nil {
			return
		}
		if len(section.Icons) == 0 {
			outErr = fmt.Errorf("section %s has no icons", section.Name)
			return
		}
		if len(section.Alias) == 0 {
			outErr = fmt.Errorf("section %s has no alias", section.Name)
			return
		}
		out = append(out, section)
		section = nil
	}
	reSpaceOrComma := regexp.MustCompile(`[ ,]`)
	splitSpace := func(s string) (out []string) {
		for _, part := range reSpaceOrComma.Split(s, -1) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			out = append(out, part)
		}
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			continue // skip empty line

		case strings.HasPrefix(line, "#"):
			continue // skip comment

		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			closeSection()

			xline := strings.TrimSpace(line[1 : len(line)-1])
			if !strings.HasPrefix(xline, "git.emoji") {
				section = nil
				continue
			}

			quotedName := strings.TrimSpace(xline[len("git.emoji"):])
			name, err := strconv.Unquote(quotedName)
			if err != nil {
				return nil, fmt.Errorf("failed to parse section: %s", line)
			}
			section = &Type{Name: name}
			continue

		default:
			if section == nil {
				continue // skip line if not in a section
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				outErr = fmt.Errorf("failed to parse line (section %q): %s", section.Name, line)
				return
			}
			directive := strings.TrimSpace(parts[0])
			switch directive {
			case "icons":
				section.Icons = append(section.Icons, splitSpace(parts[1])...)
			case "alias":
				section.Alias = append(section.Alias, splitSpace(parts[1])...)
			default:
				outErr = fmt.Errorf("unknown directive (section %q): %s", section.Name, directive)
				return
			}
		}
	}
	closeSection()
	return out, outErr
}
