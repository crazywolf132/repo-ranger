# Repo Ranger

**Repo Ranger** is an enterprise‑grade, AI‑driven GitHub Action written in Go that provides detailed, line‑by‑line code reviews. It automatically handles large diffs by summarizing changes and then reviewing them in chunks. With robust API integration, configurable timeouts, and native GitHub Checks/PR commenting (including inline review comments with code suggestions and detailed reasoning), Repo Ranger ensures your team’s code quality is maintained across your entire organization.

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
        uses: repo-ranger/repo-ranger@v1
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
git clone https://github.com/repo-ranger/repo-ranger.git
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
  uses: repo-ranger/repo-ranger@v1
  with:
    api_key: ${{ secrets.REVIEW_API_KEY }}
    api_url: 'https://your-review-api-endpoint.com'
    model: 'gpt-4'
```

### Advanced Configuration

Enable all feedback mechanisms (PR comment, inline comments, and GitHub Checks):

```yaml
- name: Repo Ranger Code Review
  uses: repo-ranger/repo-ranger@v1
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
