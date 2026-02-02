#!/bin/bash

echo "🔥 ExtAuth Match - Test Script"
echo "================================"
echo ""

# Check if services are running
echo "Checking if services are up..."
if ! docker ps | grep -q "extauth-match"; then
    echo "❌ Services not running. Please run: docker compose up --build"
    exit 1
fi

echo "✅ Services are running!"
echo ""

# Function to make a request
make_request() {
    local path=$1
    local num=$2
    echo "[$num] Making request to: $path"
    curl -s -o /dev/null -w "Status: %{http_code}\n" http://localhost:10000$path
}

echo "Making test requests to Envoy (port 10000)..."
echo "Go to http://localhost:8080 on your phone to approve/deny them!"
echo ""
echo "Sending 5 requests..."
echo ""

for i in {1..5}; do
    make_request "/test$i" "$i"
    sleep 1
done

echo ""
echo "✅ Test requests sent!"
echo ""
echo "📱 Open http://localhost:8080 (or http://<your-ip>:8080 on your phone)"
echo "👆 Swipe right to approve, left to deny!"
