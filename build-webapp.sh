#!/bin/bash

set -e

echo "Building GARM SPA (SvelteKit)..."

# Navigate to webapp directory
cd webapp

# Install dependencies if node_modules doesn't exist
npm install

# Build the SPA
echo "Building SPA..."
npm run build
echo "SPA built successfully!"
