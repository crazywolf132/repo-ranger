# Repo Ranger

**Repo Ranger** is an AI‑driven GitHub Action written in Go that provides detailed, line‑by‑line code reviews. It automatically handles large diffs by summarizing changes and then reviewing them in chunks. With robust API integration, configurable timeouts, and native GitHub Checks/PR commenting (including inline review comments with code suggestions and detailed reasoning), Repo Ranger helps maintain your team's code quality.

## Features

- **Detailed Line‑by‑Line Review:**
  Provides comments for each changed line with code suggestions and explanations.

- **Large Diff Handling:**
  Automatically summarizes very large diffs and processes them in manageable chunks.

- **Robust API Integration:**
  Built‑in retry logic, context‑based timeouts, and detailed error logging ensure reliable AI calls.

- **Multi‑Format Reporting:**
  - **Aggregated PR Comment:** Posts the full review as a developer‑friendly PR comment.
  - **Inline Comments:** Optionally posts inline review comments on the PR with code suggestions, reasoning, and explanations.
  - **GitHub Check Runs:** Optionally creates a native GitHub Check Run for integrated quality dashboards.

- **Configurable Inputs:**
  Customize diff commands, timeouts, and reporting behavior via GitHub Action inputs.

## Inputs

| Input              | Description                                                                                          | Default                | Required |
|--------------------|------------------------------------------------------------------------------------------------------|------------------------|----------|
| `api_url`          | The API endpoint URL for code review.                                                                | –                      | Yes      |
| `api_key`          | The API key for authentication with the review API.                                                  | –                      | Yes      |
| `model`            | The AI model name to use (e.g., `gpt-4`).                                                            | –                      | Yes      |
| `diff_command`     | The git diff command to run.                                                                         | `git diff HEAD~1 HEAD` | No       |
| `diff_timeout`     | Timeout (in seconds) for the diff command.                                                           | `30`                   | No       |
| `api_timeout`      | Timeout (in seconds) for each API call.                                                              | `30`                   | No       |
| `post_pr_comment`  | Whether to post the aggregated review as a PR comment (`true`/`false`).                              | `true`                 | No       |
| `use_checks`       | Whether to create a GitHub Check Run with the review output (`true`/`false`).                        | `false`                | No       |
| `inline_comments`  | Whether to post inline review comments for specific changes (`true`/`false`).                        | `false`                | No       |
| `github_token`     | A GitHub token to post PR comments, inline comments, and/or create Check Runs.                       | –                      | No       |

## Configuration

The following environment variables are used to configure the tool:

### Required Configuration
- `INPUT_API_URL`: API endpoint for code review (default: https://api.openai.com/v1/chat/completions for OpenAI)
- `INPUT_API_KEY`: API key for authentication (OpenAI API key)
- `INPUT_MODEL`: Model to use (e.g., "gpt-4", "gpt-3.5-turbo")

### Optional Configuration
- `INPUT_DIFF_COMMAND`: Command to generate diff (default: "git --no-pager diff HEAD~1 HEAD")
- `INPUT_DIFF_TIMEOUT`: Timeout in seconds for diff command (default: 30)
- `INPUT_API_TIMEOUT`: Timeout in seconds for API calls (default: 30)
- `INPUT_POST_PR_COMMENT`: Whether to post review as PR comment (default: true)
- `INPUT_USE_CHECKS`: Whether to create GitHub check runs (default: false)
- `INPUT_INLINE_COMMENTS`: Whether to post inline comments (default: false)
- `INPUT_GITHUB_TOKEN`: GitHub token for posting comments
- `INPUT_TEMPERATURE`: OpenAI temperature parameter (default: 0.7)
- `INPUT_MAX_TOKENS`: OpenAI max tokens parameter (default: 2000)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)

### OpenAI Integration

This tool now supports OpenAI's chat completion API out of the box. To use OpenAI:

1. Set `INPUT_API_KEY` to your OpenAI API key
2. Leave `INPUT_API_URL` empty to use the default OpenAI endpoint, or set it to a custom endpoint
3. Set `INPUT_MODEL` to an OpenAI model (e.g., "gpt-4" or "gpt-3.5-turbo")
4. Optionally configure `INPUT_TEMPERATURE` and `INPUT_MAX_TOKENS` to control the AI's behavior

Example configuration for OpenAI:
```bash
export INPUT_API_KEY="your-openai-api-key"
export INPUT_MODEL="gpt-4"
export INPUT_TEMPERATURE="0.7"
export INPUT_MAX_TOKENS="2000"
```

## Using Repo Ranger on Your Repository

To enable Repo Ranger on your own repository:

1. Create a `.github/workflows/repo-ranger.yml` file with the following content:
```yaml
name: Repo Ranger Code Review

on:
  pull_request:
    types: [opened, synchronize, reopened]
    paths:
      - '**.go'  # Adjust file patterns as needed
      - 'go.mod'
      - 'go.sum'

jobs:
  review:
    name: Review Code
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write
      checks: write

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run Repo Ranger
        env:
          INPUT_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          INPUT_MODEL: "gpt-4"
          INPUT_TEMPERATURE: "0.7"
          INPUT_MAX_TOKENS: "2000"
          INPUT_POST_PR_COMMENT: "true"
          INPUT_USE_CHECKS: "true"
          INPUT_INLINE_COMMENTS: "true"
          INPUT_GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          LOG_LEVEL: "debug"
        run: |
          go build -o repo-ranger
          ./repo-ranger
```

2. Add your OpenAI API key as a repository secret:
   - Go to your repository settings
   - Navigate to "Secrets and variables" → "Actions"
   - Click "New repository secret"
   - Name: `OPENAI_API_KEY`
   - Value: Your OpenAI API key

3. Configure the workflow (optional):
   - Adjust the file patterns in the `paths` section to match your needs
   - Modify the OpenAI parameters (`INPUT_MODEL`, `INPUT_TEMPERATURE`, etc.)
   - Enable/disable features using the boolean flags

Now, Repo Ranger will automatically review all pull requests that modify Go files in your repository!

## Installation

### GitHub Actions Workflow

Add Repo Ranger to your GitHub Actions workflow (`.github/workflows/code-review.yml`):

```yaml
name: Code Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0
      - name: Repo Ranger Code Review
        uses: crazywolf132/repo-ranger@v1.0.0
        with:
          api_key: ${{ secrets.REVIEW_API_KEY }}
          api_url: 'https://your-review-api-endpoint.com'
          model: 'gpt-4'
          inline_comments: true
          github_token: ${{ secrets.GITHUB_TOKEN }}
```

### Local Development

To build and run Repo Ranger locally:

1. Clone the repository:
```bash
git clone https://github.com/crazywolf132/repo-ranger.git
cd repo-ranger
```

2. Install dependencies:
```bash
go mod download
```

3. Build the binary:
```bash
go build -o repo-ranger
```

## Usage Examples

### Basic Usage

The simplest configuration posts an aggregated review comment on the PR:

```yaml
- name: Repo Ranger Code Review
  uses: crazywolf132/repo-ranger@v1.0.0
  with:
    api_key: ${{ secrets.REVIEW_API_KEY }}
    api_url: 'https://your-review-api-endpoint.com'
    model: 'gpt-4'
```

### Advanced Configuration

Enable all feedback mechanisms (PR comment, inline comments, and GitHub Checks):

```yaml
- name: Repo Ranger Code Review
  uses: crazywolf132/repo-ranger@v1.0.0
  with:
    api_key: ${{ secrets.REVIEW_API_KEY }}
    api_url: 'https://your-review-api-endpoint.com'
    model: 'gpt-4'
    post_pr_comment: true
    inline_comments: true
    use_checks: true
    github_token: ${{ secrets.GITHUB_TOKEN }}
    diff_timeout: 60
    api_timeout: 45
```

## Contributing

We welcome contributions to Repo Ranger! Here's how you can help:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure your PR:
- Includes tests for new functionality
- Updates documentation as needed
- Follows the existing code style
- Includes a clear description of the changes

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

If you encounter any issues or have questions:
- Open an issue in the GitHub repository
- Check existing issues for solutions
- Review our documentation

---
Built with ❤️ by Brayden Moon
