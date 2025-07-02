# Read version from VERSION file
VERSION := $(shell cat VERSION | tr -d '\n')

# Build the application with version injected
build:
	go build -ldflags "-X github.com/galamiram/nadctl/internal/version.Version=$(VERSION)" -o nadctl .

# Install the application
install: build
	mv nadctl /usr/local/bin/

# Clean build artifacts
clean:
	rm -f nadctl

# Test the application
test:
	go test ./...

# Run the TUI in demo mode
demo:
	./nadctl tui --demo

# Show current version (after building)
version: build
	./nadctl version

# Create git tag and push release using VERSION file
release:
	@echo "Creating release for version $(VERSION)"
	@git tag v$(VERSION) -f
	@git push origin v$(VERSION) -f
	@echo "Successfully released v$(VERSION)"

# Generate changelog entry using Cursor LLM based on git changes
changelog:
	@echo "🤖 Generating AI-powered changelog entry for version $(VERSION)..."
	@LAST_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "HEAD~10"); \
	echo "📊 Analyzing changes since $$LAST_TAG..."; \
	COMMIT_MSGS=$$(git log --oneline --pretty=format:"- %s" $$LAST_TAG..HEAD 2>/dev/null || git log --oneline --pretty=format:"- %s" -n 10); \
	FILE_CHANGES=$$(git diff --name-status $$LAST_TAG..HEAD 2>/dev/null || git diff --name-status HEAD~10..HEAD); \
	CODE_DIFF=$$(git diff $$LAST_TAG..HEAD 2>/dev/null | head -200 || git diff HEAD~10..HEAD | head -200); \
	DATE=$$(date +%Y-%m-%d); \
	PROMPT_FILE="/tmp/changelog_prompt_$(VERSION).md"; \
	echo "# Changelog Generation Request" > $$PROMPT_FILE; \
	echo "" >> $$PROMPT_FILE; \
	echo "Please analyze the following git changes and generate a professional changelog entry for version $(VERSION)." >> $$PROMPT_FILE; \
	echo "" >> $$PROMPT_FILE; \
	echo "## Version Information" >> $$PROMPT_FILE; \
	echo "- Version: $(VERSION)" >> $$PROMPT_FILE; \
	echo "- Date: $$DATE" >> $$PROMPT_FILE; \
	echo "- Previous tag: $$LAST_TAG" >> $$PROMPT_FILE; \
	echo "" >> $$PROMPT_FILE; \
	echo "## Commit Messages" >> $$PROMPT_FILE; \
	echo "\`\`\`" >> $$PROMPT_FILE; \
	echo "$$COMMIT_MSGS" >> $$PROMPT_FILE; \
	echo "\`\`\`" >> $$PROMPT_FILE; \
	echo "" >> $$PROMPT_FILE; \
	echo "## File Changes" >> $$PROMPT_FILE; \
	echo "\`\`\`" >> $$PROMPT_FILE; \
	echo "$$FILE_CHANGES" >> $$PROMPT_FILE; \
	echo "\`\`\`" >> $$PROMPT_FILE; \
	echo "" >> $$PROMPT_FILE; \
	echo "## Code Diff Sample" >> $$PROMPT_FILE; \
	echo "\`\`\`diff" >> $$PROMPT_FILE; \
	echo "$$CODE_DIFF" >> $$PROMPT_FILE; \
	echo "\`\`\`" >> $$PROMPT_FILE; \
	echo "" >> $$PROMPT_FILE; \
	echo "## Instructions" >> $$PROMPT_FILE; \
	echo "1. Add a new changelog section at the top of CHANGELOG.md with the format:" >> $$PROMPT_FILE; \
	echo "   \`## [$(VERSION)] - $$DATE\`" >> $$PROMPT_FILE; \
	echo "2. Organize changes into appropriate categories:" >> $$PROMPT_FILE; \
	echo "   - **Added**: New features and capabilities" >> $$PROMPT_FILE; \
	echo "   - **Changed**: Changes to existing functionality" >> $$PROMPT_FILE; \
	echo "   - **Fixed**: Bug fixes and corrections" >> $$PROMPT_FILE; \
	echo "   - **Removed**: Removed features or deprecated functionality" >> $$PROMPT_FILE; \
	echo "   - **Technical Details**: Implementation details for developers" >> $$PROMPT_FILE; \
	echo "3. Write user-friendly descriptions, not just technical commit messages" >> $$PROMPT_FILE; \
	echo "4. Focus on user-visible changes and benefits" >> $$PROMPT_FILE; \
	echo "5. Group related changes together logically" >> $$PROMPT_FILE; \
	echo "" >> $$PROMPT_FILE; \
	echo "Please update CHANGELOG.md by adding this new section at the very top, right after the '# Changelog' header." >> $$PROMPT_FILE; \
	if command -v cursor >/dev/null 2>&1; then \
		echo "🚀 Invoking Cursor AI to generate changelog..."; \
		if cursor --apply "$$(<$$PROMPT_FILE)" CHANGELOG.md 2>/dev/null; then \
			echo "✅ Changelog automatically generated and applied!"; \
			echo "📝 Please review CHANGELOG.md to ensure accuracy"; \
		else \
			echo "⚠️  Cursor auto-apply failed, trying interactive mode..."; \
			if cursor --chat "$$(<$$PROMPT_FILE)" 2>/dev/null; then \
				echo "💬 Opened Cursor chat with changelog prompt"; \
				echo "📄 Prompt also saved to: $$PROMPT_FILE"; \
			else \
				echo "❌ Cursor command failed. Manual fallback:"; \
				echo "📖 Please copy the following prompt to Cursor:"; \
				echo ""; \
				cat $$PROMPT_FILE; \
			fi; \
		fi; \
	else \
		echo "⚠️  Cursor CLI not found"; \
		echo "📥 Install with: npm install -g cursor-cli"; \
		echo "📄 Generated prompt saved to: $$PROMPT_FILE"; \
		echo ""; \
		echo "📋 Manual prompt (copy to Cursor):"; \
		echo "=========================================="; \
		cat $$PROMPT_FILE; \
		echo "=========================================="; \
	fi

# Simple automated changelog (fallback without AI)
changelog-simple:
	@echo "📝 Generating simple changelog entry for version $(VERSION)..."
	@LAST_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "HEAD~10"); \
	CHANGES=$$(git log --oneline --pretty=format:"- %s" $$LAST_TAG..HEAD 2>/dev/null || git log --oneline --pretty=format:"- %s" -n 10); \
	DATE=$$(date +%Y-%m-%d); \
	TEMP_FILE=$$(mktemp); \
	echo "## [$(VERSION)] - $$DATE" > $$TEMP_FILE; \
	echo "" >> $$TEMP_FILE; \
	echo "### Changes" >> $$TEMP_FILE; \
	echo "$$CHANGES" >> $$TEMP_FILE; \
	echo "" >> $$TEMP_FILE; \
	echo "### Technical Details" >> $$TEMP_FILE; \
	git diff --name-only $$LAST_TAG..HEAD 2>/dev/null | sed 's/^/- Updated: /' >> $$TEMP_FILE || git diff --name-only HEAD~10..HEAD | sed 's/^/- Updated: /' >> $$TEMP_FILE; \
	echo "" >> $$TEMP_FILE; \
	echo "" >> $$TEMP_FILE; \
	cat CHANGELOG.md >> $$TEMP_FILE; \
	mv $$TEMP_FILE CHANGELOG.md; \
	echo "✅ Simple changelog entry added for version $(VERSION)"; \
	echo "💡 Run 'make changelog' for AI-enhanced descriptions"

.PHONY: build install clean test demo version release changelog changelog-simple 