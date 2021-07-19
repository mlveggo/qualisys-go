golangci_lint_cwd := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
golangci_lint_version := 1.42.1
golangci_lint := $(golangci_lint_cwd)/$(golangci_lint_version)/golangci-lint

system_os := $(shell uname -s | tr A-Z a-z)
system_arch := $(shell uname -m | sed 's/x86_/amd/')

ifneq ($(shell uname),Linux)
ifneq ($(shell uname),Darwin)
$(error unsupported OS: $(shell uname))
endif
endif

golangci_lint_archive_url := https://github.com/golangci/golangci-lint/releases/download/v${golangci_lint_version}/golangci-lint-${golangci_lint_version}-$(system_os)-$(system_arch).tar.gz

$(golangci_lint):
	$(info [golangci-lint] fetching $(golangci_lint_version) binary...)
	@mkdir -p $(dir $@)
	@curl -sSL $(golangci_lint_archive_url) -o - | \
			tar -xz --directory $(dir $@) --strip-components 1
	@chmod +x $@
	@touch $@

.PHONY: go-lint
go-lint: $(golangci_lint)
	$(info [$@] linting Go code...)
	@$(golangci_lint) run
