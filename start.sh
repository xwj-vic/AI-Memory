#!/bin/bash

# Exit on error
set -e

echo "=== Building Frontend ==="
cd frontend
npm install
npm run build
cd ..

echo "=== Starting Backend ==="
echo "Serving frontend from ./frontend/dist"
echo "Admin Dashboard available at http://localhost:8080"
go run main.go
