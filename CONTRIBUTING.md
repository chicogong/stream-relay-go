# Contributing to Stream Relay Go

Thank you for your interest in contributing to Stream Relay Go! This document provides guidelines and instructions for contributing.

## 🎯 How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. When creating a bug report, include:

- **Clear title** - Brief, descriptive summary
- **Steps to reproduce** - Exact steps to reproduce the issue
- **Expected behavior** - What you expected to happen
- **Actual behavior** - What actually happened
- **Environment** - OS, Go version, Docker version
- **Logs** - Relevant error messages or logs

### Suggesting Enhancements

Enhancement suggestions are welcome! Please include:

- **Use case** - Why this enhancement would be useful
- **Proposed solution** - How you envision it working
- **Alternatives** - Other solutions you've considered

### Pull Requests

1. **Fork the repository** and create your branch from `master`
2. **Make your changes** following our coding standards
3. **Add tests** if you're adding functionality
4. **Update documentation** if needed
5. **Ensure tests pass** with `make test`
6. **Format your code** with `make fmt`
7. **Create a pull request** with a clear description

## 💻 Development Setup

### Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- Git

### Setup Steps

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/stream-relay-go.git
cd stream-relay-go

# Add upstream remote
git remote add upstream https://github.com/chicogong/stream-relay-go.git

# Install dependencies
go mod download

# Build
make build

# Run tests
make test
```

## 📝 Coding Standards

### Go Style Guide

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Keep functions small and focused
- Write clear, descriptive variable names
- Add comments for exported functions

### Code Organization

```
internal/
├── config.go      # Configuration loading and validation
├── proxy.go       # Core proxy logic
├── server.go      # HTTP server setup
├── metrics.go     # Prometheus metrics
├── limiter.go     # Rate limiting
└── storage.go     # Storage layer
```

### Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

Examples:
```
feat(proxy): add support for WebSocket streaming
fix(metrics): correct duration calculation for long requests
docs: update installation instructions
```

## 🧪 Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific test
go test -v ./internal -run TestProxyHandle
```

### Writing Tests

- Place tests in `*_test.go` files
- Use table-driven tests for multiple cases
- Mock external dependencies
- Aim for >80% code coverage

Example:
```go
func TestProxy_Handle(t *testing.T) {
    tests := []struct {
        name    string
        route   *RouteConfig
        want    int
        wantErr bool
    }{
        {
            name: "successful request",
            route: &RouteConfig{...},
            want: 200,
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## 📚 Documentation

### Code Documentation

- Add godoc comments for all exported functions
- Include examples where helpful
- Keep comments up to date with code changes

### README Updates

Update README.md if you:
- Add new features
- Change configuration options
- Modify installation steps
- Add new dependencies

## 🔍 Code Review Process

1. **Automated checks** must pass (tests, linting)
2. **At least one maintainer** must approve
3. **All comments** must be addressed
4. **No merge conflicts** with master branch

## 🎨 Grafana Dashboard Contributions

When contributing dashboard changes:

1. Export dashboard as JSON
2. Place in `deployments/grafana/`
3. Update documentation
4. Include screenshot of changes

## 🐛 Debugging

### Enable Debug Logging

```yaml
# configs/config.yaml
observability:
  logging:
    level: debug
```

### Common Issues

**Import paths:**
```go
// Use internal imports
import "github.com/chicogong/stream-relay-go/internal"
```

**Docker networking:**
- Use `host.docker.internal` on macOS/Windows
- Use actual IP on Linux

## 📋 Checklist

Before submitting your PR:

- [ ] Code follows Go style guide
- [ ] Tests added for new functionality
- [ ] All tests pass
- [ ] Documentation updated
- [ ] Commit messages follow convention
- [ ] No sensitive information committed
- [ ] Branch is up to date with master

## 🤝 Code of Conduct

### Our Pledge

We pledge to make participation in our project a harassment-free experience for everyone.

### Our Standards

- **Be respectful** of differing viewpoints
- **Be collaborative** and constructive
- **Focus on what is best** for the community
- **Show empathy** towards other community members

## 📞 Getting Help

- 💬 [GitHub Discussions](https://github.com/chicogong/stream-relay-go/discussions)
- 🐛 [Issue Tracker](https://github.com/chicogong/stream-relay-go/issues)
- 📧 Email: your-email@example.com

## 📜 License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing! 🎉
