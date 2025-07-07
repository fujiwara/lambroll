# Development Guidelines for lambroll

This document provides guidelines for developing lambroll with AI assistance.

## Code Style and Conventions

- Follow existing code patterns and conventions in the codebase
- Add tests for new functionality whenever possible
- Use meaningful commit messages that describe the changes
- Keep changes focused and atomic

## Testing

- Always run tests before committing: `go test ./...`
- Add unit tests for new features
- Test both positive and negative cases
- Verify backward compatibility when making changes

## Git Workflow

- **Never commit directly to main/master/v1 branches** - always create a feature branch first
- Create a new branch for each feature or fix
- Use descriptive branch names (e.g., `fix-ext-str-compatibility`)
- When committing, use `git add` with specific files rather than `git add -A`
  - Only add files that are directly related to your change
  - Review `git status` to see all modified files
  - Add files individually: `git add file1.go file2_test.go`
  - This prevents accidentally committing unrelated changes
- Include clear commit messages that explain the "why" not just the "what"

## Pull Requests

- Create PRs with clear descriptions of the problem and solution
- Include test plan and verification steps
- Link related issues in the PR description
- Ensure all tests pass before requesting review

## Backward Compatibility

- Maintain backward compatibility when possible
- Add deprecation warnings for features that will be removed
- Document migration paths for breaking changes
- Use TODO comments to track future removals (e.g., `TODO: Remove in v2`)

## Documentation

- Update documentation when adding new features
- Include examples in documentation
- Keep README and other docs in sync with code changes

## Common Tasks

### Adding a new feature
1. Create a feature branch
2. Implement the feature with tests
3. Run linting and tests
4. Create a PR with detailed description

### Fixing a bug
1. Reproduce the issue
2. Create a test that demonstrates the bug
3. Fix the issue
4. Verify the test passes
5. Check for regression with existing tests

### Refactoring
1. Ensure comprehensive test coverage exists
2. Make incremental changes
3. Run tests after each change
4. Keep commits atomic and focused

## Project-Specific Notes

- lambroll uses Jsonnet for configuration templating
- Option files can use both JSON and Jsonnet formats
- External variables (`ext_str`/`ext_code`) are only used with Jsonnet files
- The project maintains compatibility with AWS Lambda's evolving API

## AI Assistance Tips

When working with AI assistants:
- Be specific about requirements and constraints
- Review generated code carefully
- Test all changes thoroughly
- Don't rely solely on AI-generated tests - add your own edge cases
- Use AI for exploration and initial implementation, but apply your domain knowledge

Remember: AI is a tool to enhance productivity, not replace careful engineering practices.