#!/bin/bash

# Create a temporary directory for testing
TEST_DIR=$(mktemp -d)
cp ./repo-ranger "$TEST_DIR/"
cd "$TEST_DIR"

# Initialize a new git repository
git init
git config --local user.email "test@example.com"
git config --local user.name "Test User"
# Disable git pager
git config --local core.pager cat

# Set environment variables for testing
export INPUT_API_KEY="your-openai-api-key"
export INPUT_API_URL="https://api.openai.com/v1/chat/completions"
export INPUT_MODEL="gpt-4"
export INPUT_TEMPERATURE="0.7"
export INPUT_MAX_TOKENS="2000"
export INPUT_POST_PR_COMMENT="true"
export INPUT_USE_CHECKS="true"
export INPUT_INLINE_COMMENTS="true"
export INPUT_GITHUB_TOKEN="your-github-token"
export LOG_LEVEL="debug"

# Create initial commit
touch README.md
git add README.md
git commit -m "chore: initial commit"

# Create a test branch and some changes
git checkout -b test-branch

# Create a test Go file with some code that could be reviewed
cat > test.go << 'EOL'
package main

import "fmt"

func main() {
    // This is intentionally not using a constant
    x := "Hello World"
    fmt.Println(x)
    
    // This loop could be improved
    for i := 0; i < 10; i++ {
        if i % 2 == 0 {
            fmt.Println(i)
        }
    }
    
    // This function call is missing error handling
    result := someFunction()
    fmt.Println(result)
}

func someFunction() string {
    // This could use better variable naming
    a := "test"
    return a
}
EOL

git add test.go
git commit -m "test: Add sample Go code with review opportunities"

# Set the diff command to compare against the parent branch
export INPUT_DIFF_COMMAND="git --no-pager diff main..test-branch"

# Show what changes will be reviewed
echo "Changes to be reviewed:"
eval $INPUT_DIFF_COMMAND

# Run repo-ranger
./repo-ranger

# Clean up
cd -
rm -rf "$TEST_DIR" 