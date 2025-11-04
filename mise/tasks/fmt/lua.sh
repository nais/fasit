#!/usr/bin/env bash
#MISE description="Format lua code"
set -euo pipefail

LUA_FORMATTER_VERSION=1.5.6
BIN_DIR=./bin
LUAFMT="$BIN_DIR/luafmt-$LUA_FORMATTER_VERSION"
LUA_FORMATTER_URL="https://github.com/CppCXY/EmmyLuaCodeStyle/releases/download/$LUA_FORMATTER_VERSION"

if [ ! -d "$LUAFMT" ]; then
	OS=$(uname -s)
	ARCH=$(uname -m)
	if [ "$OS" = "Darwin" ]; then
		if [ "$ARCH" = "x86_64" ]; then
			LUA_FORMATTER_FILE=darwin-x64
		else
			if [ "$ARCH" = "arm64" ]; then
				LUA_FORMATTER_FILE=darwin-arm64
			else
				echo "Unsupported architecture: $ARCH on macOS"
				exit 1
			fi
		fi
	elif [ "$OS" = "Linux" ]; then
		if [ "$ARCH" = "x86_64" ]; then
			LUA_FORMATTER_FILE=linux-x64
		else
			if [ "$ARCH" = "aarch64" ]; then
				LUA_FORMATTER_FILE=linux-aarch64
			else
				echo "Unsupported architecture: $ARCH on Linux"
				exit 1
			fi
		fi
	else
		echo "Unsupported OS: $OS"
		exit 1
	fi

	mkdir -p "$LUAFMT"
	curl -L "$LUA_FORMATTER_URL/$LUA_FORMATTER_FILE.tar.gz" -o /tmp/luafmt.tar.gz
	tar -xzf /tmp/luafmt.tar.gz -C "$LUAFMT"
	rm /tmp/luafmt.tar.gz
	mv "$LUAFMT/$LUA_FORMATTER_FILE/"* "$LUAFMT/"
	rmdir "$LUAFMT/$LUA_FORMATTER_FILE"
fi

$LUAFMT/bin/CodeFormat format -w . --ignores-file ".gitignore" -c ./integration_tests/.editorconfig
