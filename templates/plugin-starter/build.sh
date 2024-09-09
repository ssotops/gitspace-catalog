#!/bin/bash

set -e

echo "Building plugin-starter..."
go build -buildmode=plugin -o plugin-starter.so .

echo "Build complete!"
