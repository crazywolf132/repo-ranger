#!/bin/bash

# Set up test environment variables
export INPUT_API_URL="https://api.example.com/review"
export INPUT_API_KEY="test_key"
export INPUT_MODEL="gpt-4"
export INPUT_DIFF_COMMAND="git --no-pager diff HEAD~1 HEAD"
export INPUT_DIFF_TIMEOUT="30"
export INPUT_API_TIMEOUT="30"
export INPUT_POST_PR_COMMENT="true"
export INPUT_USE_CHECKS="true"
export INPUT_INLINE_COMMENTS="true"
export INPUT_GITHUB_TOKEN="test_token"
export GITHUB_EVENT_PATH="test_event.json"
export LOG_LEVEL="debug"

# Create a test event file
cat > test_event.json << EOL
{
  "pull_request": {
    "number": 123
  },
  "repository": {
    "full_name": "test/repo"
  }
}
EOL

# Make a test change
echo "// Test change" >> main.go
git add main.go

# Run the tool
./repo-ranger

# Clean up
rm test_event.json
git checkout main.go
