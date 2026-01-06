#!/usr/bin/env bash
#MISE description="Ensure all code is formatted"
#MISE depends=["generate", "fmt"]

if ! git diff --exit-code --name-only; then
	echo "The file(s) listed above are not formatted correctly. Please run \`mise run fmt\` and commit the changes."
	git diff
	exit 1
fi
