#!/bin/bash

v1_count=0
v2_count=0
total=100

for i in $(seq 1 $total); do
  echo "Request $i of $total"
  response=$(curl -s https://istio-test.rajesh-kumar.in/api/info)
  version=$(echo $response | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
  
  if [ "$version" == "v1" ]; then
    v1_count=$((v1_count + 1))
  elif [ "$version" == "v2" ]; then
    v2_count=$((v2_count + 1))
  fi
  
  echo "Current counts - v1: $v1_count, v2: $v2_count"
  sleep 0.2
done

echo "===== RESULTS ====="
echo "Total requests: $total"
echo "v1 count: $v1_count ($(( v1_count * 100 / total ))%)"
echo "v2 count: $v2_count ($(( v2_count * 100 / total ))%)"
