#!/usr/bin/env bash
# Adapter: goose -> LM Studio (local, port 1234)
set -euo pipefail

export GOOSE_PROVIDER=openai
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_API_KEY="not-needed"
export GOOSE_MODEL="qwen/qwen3.6-35b-a3b"

exec goose run -i - \
  --with-builtin developer \
  --no-session \
  --max-turns 30
