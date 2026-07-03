.PHONY: all build wizard clean

VERSION ?= 1.0.0
BUILD_DIR = packer/output

all: wizard

wizard:
	go build -ldflags="-s -w" -o netsoft-wizard ./cmd/wizard/
	strip netsoft-wizard
	@echo "Wizard binary: $$(ls -lh netsoft-wizard | awk '{print $$5}')"

build: wizard
	cd packer && packer build -var 'version=$(VERSION)' -var 'wizard_binary=../netsoft-wizard' ubuntu-24.04.pkr.hcl
	@echo "OVA: $(BUILD_DIR)/netsoft-ztna-$(VERSION)/netsoft-ztna-$(VERSION).ova"

clean:
	rm -f netsoft-wizard
	rm -rf $(BUILD_DIR)
	@echo "Cleaned"

run: wizard
	NETSOFT_WEB_DIR=./web ./netsoft-wizard

list:
	@find $(BUILD_DIR) -name '*.ova' -exec ls -lh {} \; 2>/dev/null || echo "No OVA files found"
