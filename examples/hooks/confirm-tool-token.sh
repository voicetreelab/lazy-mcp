#!/bin/bash
#
# Token-based Confirmation Hook for lazy-mcp
#
# This hook implements a two-phase confirmation:
# 1. First call: No token exists → Deny with details (AI shows user)
# 2. User confirms → AI creates token file
# 3. Second call: Token exists + valid → Allow and delete token
#
# Token file: /tmp/lazy-mcp-confirm-<toolpath-hash>
# Token validity: 60 seconds (prevents stale confirmations)
#
# Usage in config-agentic.json:
#   "permissions": {
#     "sensitive": ["gmail.send_email", ...],
#     "confirmationHook": "/path/to/confirm-tool-token.sh",
#     "confirmationTimeout": 10
#   }
#

set -euo pipefail

TOKEN_DIR="/home/b-codes/agentic.ai.forge/.opencode/tmp/lazy-mcp-tokens"
TOKEN_MAX_AGE=60  # seconds

# Ensure token directory exists
mkdir -p "$TOKEN_DIR"

# Read JSON from stdin
INPUT=$(cat)

# Parse tool details
TOOL_PATH=$(echo "$INPUT" | jq -r '.tool_path // "unknown"')
REASON=$(echo "$INPUT" | jq -r '.reason // "This action requires confirmation"')

# Create a hash of the tool path for the token filename
# Using md5sum for short, consistent filenames
TOOL_HASH=$(echo -n "$TOOL_PATH" | md5sum | cut -d' ' -f1)
TOKEN_FILE="$TOKEN_DIR/confirm-$TOOL_HASH"

# Check if valid token exists
if [ -f "$TOKEN_FILE" ]; then
    # Check token age
    TOKEN_AGE=$(( $(date +%s) - $(stat -c %Y "$TOKEN_FILE") ))
    
    if [ "$TOKEN_AGE" -lt "$TOKEN_MAX_AGE" ]; then
        # Valid token - allow and delete
        rm -f "$TOKEN_FILE"
        echo "Token valid, execution allowed" >&2
        exit 0
    else
        # Token too old - delete and deny
        rm -f "$TOKEN_FILE"
        echo "Token expired (${TOKEN_AGE}s old), requesting new confirmation" >&2
    fi
fi

# No valid token - output details for the AI to show user
# Format arguments nicely (truncate if too long)
ARGS=$(echo "$INPUT" | jq -r '.arguments // {}')
ARGS_PREVIEW=$(echo "$ARGS" | jq -c '.' | head -c 500)
if [ ${#ARGS_PREVIEW} -ge 500 ]; then
    ARGS_PREVIEW="${ARGS_PREVIEW}..."
fi

# Write pending request info (for debugging/logging)
echo "$INPUT" > "$TOKEN_DIR/pending-$TOOL_HASH.json"

# Exit with error and provide info for AI
cat >&2 << EOF
CONFIRMATION_NEEDED
tool_path: $TOOL_PATH
token_file: $TOKEN_FILE
arguments: $ARGS_PREVIEW
EOF

exit 1
