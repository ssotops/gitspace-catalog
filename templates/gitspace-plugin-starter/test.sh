#!/bin/bash

set -e

echo "Running tests for plugin-starter..."
go test -v .

echo "Tests complete!"
