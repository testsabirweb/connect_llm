#!/bin/bash
# Setup script for pre-commit hooks

set -e

echo "🔧 Setting up pre-commit hooks for ConnectLLM..."

# Check if pre-commit is installed
if ! command -v pre-commit &> /dev/null; then
    echo "📦 Installing pre-commit..."
    if command -v pip &> /dev/null; then
        pip install pre-commit
    elif command -v brew &> /dev/null; then
        brew install pre-commit
    else
        echo "❌ Please install pre-commit manually: https://pre-commit.com/#install"
        exit 1
    fi
fi

# Install gitleaks if not present
if ! command -v gitleaks &> /dev/null; then
    echo "🔒 Installing gitleaks..."
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        # Linux
        wget -q https://github.com/gitleaks/gitleaks/releases/download/v8.18.4/gitleaks_8.18.4_linux_x64.tar.gz
        tar -xzf gitleaks_8.18.4_linux_x64.tar.gz
        sudo mv gitleaks /usr/local/bin/
        rm gitleaks_8.18.4_linux_x64.tar.gz
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        if command -v brew &> /dev/null; then
            brew install gitleaks
        else
            echo "❌ Please install Homebrew or gitleaks manually"
            exit 1
        fi
    else
        echo "❌ Please install gitleaks manually: https://github.com/gitleaks/gitleaks#installing"
        exit 1
    fi
fi

# Install golangci-lint if not present
if ! command -v golangci-lint &> /dev/null; then
    echo "🐹 Installing golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.61.0
fi

# Install hadolint if Docker development is needed
if command -v docker &> /dev/null && ! command -v hadolint &> /dev/null; then
    echo "🐳 Installing hadolint for Docker linting..."
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        wget -q https://github.com/hadolint/hadolint/releases/download/v2.13.0/hadolint-Linux-x86_64
        chmod +x hadolint-Linux-x86_64
        sudo mv hadolint-Linux-x86_64 /usr/local/bin/hadolint
    elif [[ "$OSTYPE" == "darwin"* ]] && command -v brew &> /dev/null; then
        brew install hadolint
    fi
fi

# Install the git hooks
echo "🪝 Installing pre-commit hooks..."
pre-commit install

# Run hooks on all files to check current state
echo "🧪 Testing pre-commit hooks on all files..."
pre-commit run --all-files || true

echo "✅ Pre-commit setup complete!"
echo ""
echo "The following hooks are now active:"
echo "  - 🔒 Gitleaks: Prevents secrets from being committed"
echo "  - 🐹 Go tools: fmt, vet, mod tidy, tests, and linting"
echo "  - 📝 File checks: YAML, JSON, trailing whitespace, etc."
echo "  - 🐳 Docker: Dockerfile linting (if Docker is installed)"
echo ""
echo "To bypass hooks in emergencies, use: git commit --no-verify"
echo "To run hooks manually: pre-commit run --all-files"
