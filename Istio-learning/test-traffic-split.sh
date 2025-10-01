#!/bin/bash

v1_count=0
v2_count=0
total=100
unknown_count=0

echo "Starting traffic split test with $total requests..."

for i in $(seq 1 $total); do
  echo "Request $i of $total"
  
  # Get the JSON response from the API endpoint
  response=$(curl -s https://istio-test.rajesh-kumar.in/api/data)
  
  # Extract the version from the JSON response
  version=$(echo "$response" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
  
  if [ "$version" == "v1" ]; then
    v1_count=$((v1_count + 1))
    echo "Detected version: v1"
  elif [ "$version" == "v2" ]; then
    v2_count=$((v2_count + 1))
    echo "Detected version: v2"
  else
    unknown_count=$((unknown_count + 1))
    echo "Detected version: UNKNOWN"
  fi
  
  echo "Current counts - v1: $v1_count, v2: $v2_count, unknown: $unknown_count"
  sleep 0.5  # Slight delay between requests
done

echo "===== RESULTS ====="
echo "Total requests: $total"
echo "v1 count: $v1_count ($(( v1_count * 100 / total ))%)"
echo "v2 count: $v2_count ($(( v2_count * 100 / total ))%)"
echo "unknown count: $unknown_count ($(( unknown_count * 100 / total ))%)"

# Check if the distribution is close to expected (80/20 split)
# kubectl -n demo-app get httproute -o json | jq '.items[] | select(.spec.hostnames == null or (.spec.hostnames | length == 0)) | .metadata.name' -r
v1_expected=80
v2_expected=20
v1_actual=$(( v1_count * 100 / total ))
v2_actual=$(( v2_count * 100 / total ))

v1_diff=$(( v1_actual > v1_expected ? v1_actual - v1_expected : v1_expected - v1_actual ))
v2_diff=$(( v2_actual > v2_expected ? v2_actual - v2_expected : v2_expected - v2_actual ))

echo ""
echo "===== ANALYSIS ====="
echo "Expected distribution: v1: ${v1_expected}%, v2: ${v2_expected}%"
echo "Actual distribution:   v1: ${v1_actual}%, v2: ${v2_actual}%"
echo "Deviation:             v1: ${v1_diff}%, v2: ${v2_diff}%"

# Determine if the test passed (within 10% margin of error)
margin=10
if [ $v1_diff -le $margin ] && [ $v2_diff -le $margin ] && [ $unknown_count -eq 0 ]; then
  echo "TEST PASSED  - Traffic distribution is within ${margin}% of expected values"
else
  echo "TEST FAILED - Traffic distribution deviates by more than ${margin}% from expected values or unknown responses detected"
fi
