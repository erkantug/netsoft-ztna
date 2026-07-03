.PHONY: all build wizard clean

VERSION ?= 1.0.0
BUILD_DIR = packer/output

all: wizard

# Build the Go wizard binary
wizard:
	go build -ldflags="-s -w" -o netsoft-wizard ./cmd/wizard/
	strip netsoft-wizard
	@echo "✅ Wizard binary: $$(ls -lh netsoft-wizard | awk '{print $$5}')"

# Build the OVA via Packer
build: wizard
	cd packer && packer build -var 'version=$(VERSION)' -var 'wizard_binary=../netsoft-wizard' ubuntu-24.04.pkr.hcl
	@echo "✅ OVA built: $(BUILD_DIR)/netsoft-ztna-$(VERSION).ova"

# Clean build artifacts
clean:
	rm -f netsoft-wizard
	rm -rf $(BUILD_DIR)
	@echo "✅ Cleaned"

# Quick test: run wizard locally
test-run: wizard
	@echo "Starting wizard on :8080..."
	NETSOFT_WEB_DIR=./web ./netsoft-wizard

# List OVA output
list:
	@ls -lh $(BUILD_DIR)/*.ova 2>/dev/null || echo "No OVA files found"
