#!/usr/bin/env bash
# Upload a release archive to VirusTotal API v3 and gate on detections.
set -euo pipefail

file=${1:?usage: virustotal-scan.sh FILE}
if [[ ! -f "$file" ]]; then
  echo "VirusTotal input not found: $file" >&2
  exit 2
fi
if [[ -z ${VT_API_KEY:-} ]]; then
  if [[ ${REQUIRE_VT:-0} == 1 ]]; then
    echo "VT_API_KEY is required" >&2
    exit 2
  fi
  echo "VirusTotal scan skipped: VT_API_KEY is not configured"
  exit 0
fi

api=https://www.virustotal.com/api/v3
size=$(wc -c < "$file")
upload="$api/files"
if (( size > 32000000 )); then
  response=$(curl --fail --silent --show-error -H "x-apikey: $VT_API_KEY" "$api/files/upload_url")
  upload=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"])' <<<"$response")
fi
response=$(curl --fail --silent --show-error -X POST -H "x-apikey: $VT_API_KEY" -F "file=@$file" "$upload")
analysis=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["id"])' <<<"$response")
echo "VirusTotal analysis submitted: $analysis"

for attempt in $(seq 1 30); do
  response=$(curl --fail --silent --show-error -H "x-apikey: $VT_API_KEY" "$api/analyses/$analysis")
  status=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["attributes"]["status"])' <<<"$response")
  if [[ $status == completed ]]; then
    read -r malicious suspicious < <(python3 -c 'import json,sys; s=json.load(sys.stdin)["data"]["attributes"]["stats"]; print(s.get("malicious",0), s.get("suspicious",0))' <<<"$response")
    echo "VirusTotal result: malicious=$malicious suspicious=$suspicious"
    if (( malicious > ${VT_MAX_MALICIOUS:-0} || suspicious > ${VT_MAX_SUSPICIOUS:-0} )); then
      echo "VirusTotal release gate failed" >&2
      exit 1
    fi
    exit 0
  fi
  sleep 20
done
echo "VirusTotal analysis did not complete within 10 minutes" >&2
exit 1
