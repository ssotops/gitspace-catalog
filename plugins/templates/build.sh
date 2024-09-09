#!/bin/bash

set -e

echo "Building gitspace-plugin-templates..."
go build -buildmode=plugin -o gitspace-plugin-templates.so .

echo "Build complete!"
