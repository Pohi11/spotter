#!/bin/sh
# Writes the S3 identity from environment variables at start, so no credentials live in the repo,
# then runs SeaweedFS in single-process mode (master, volume, filer, and S3 gateway together).
set -eu

cat > /tmp/s3.json <<JSON
{
  "identities": [
    {
      "name": "spotter",
      "credentials": [{ "accessKey": "${S3_ACCESS_KEY}", "secretKey": "${S3_SECRET_KEY}" }],
      "actions": ["Admin", "Read", "List", "Tagging", "Write"]
    }
  ]
}
JSON

exec weed server -dir=/data -s3 -s3.port=8333 -s3.config=/tmp/s3.json -volume.max=0 -master.volumeSizeLimitMB=1024
