#!/bin/bash

set -euo pipefail

feature=$1
version=$2

if [ -z "$feature" ] || [ -z "$version" ]; then
    echo "Usage: $0 <feature> <version>"
    exit 1
fi

if [ ! -f features/$feature.yaml ]; then
    echo "Feature $feature does not exist"
    exit 1
fi

sed -i "s/^version:.*/version: $version/" features/$feature.yaml

git add features/$feature.yaml
git commit -m "$feature: update version to $version"
git push

