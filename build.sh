#!/bin/bash
set -eo pipefail

if ! go version &> /dev/null; then
    echo "Go is not installed. Please install Go first."
    echo
    echo "👉 https://go.dev/doc/install"
    exit 1
fi

# cd to the root directory
cd $(git rev-parse --show-toplevel)

# install to $GOPATH/bin
go install .

# get go bin path
GOBIN=$(go env GOPATH)/bin

# add to PATH
echo "✅ git.emoji installed at $GOBIN/git.emoji"
echo

# do you want to setup git hooks?
echo "👉 do you want to setup git hooks? (y/n)"
read -r answer
if [ "$answer" = "y" ]; then
    echo "👉 git.emoji setup-hooks"
    "$GOBIN/git.emoji" setup-hooks
fi

# do you want to alias git to git.emoji?
echo
echo "👉 to use git.emoji as git alias, add this to your .zshrc or .bashrc:"
echo
echo "   alias git=$GOBIN/git.emoji"
