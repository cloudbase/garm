#!/bin/bash

latest=$(git describe --tags --match='v[0-9]*' --abbrev=0)
IFS='.' read -r major minor patch <<< "${latest#v}"
patch=$((patch + 1))
next="v$major.$minor.$patch"
commit_info=$(git describe --tags --match='v[0-9]*' --dirty --always)

if [[ "$latest" == "$commit_info" ]]; then
    echo "$latest"
else
    echo "${next}-${commit_info#${latest}-}"
fi
