#!/bin/bash

set -e

echo "Running tests for gitspace-plugin-templates..."
go test -v .

echo "Tests complete!"
