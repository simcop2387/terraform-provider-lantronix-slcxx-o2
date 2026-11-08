BINARY      := dist/terraform-provider-lantronix-slcxx-o2
DEVTFRC     := lab/dev.tfrc
TERRAFORM   ?= /usr/bin/terraform
TOFU        ?= $(shell command -v tofu || echo /usr/bin/tofu)

.PHONY: build test lint harness acc-tf acc-tofu smoke docs tidy

build:
	go build -o $(BINARY) .

test:
	go test ./internal/... -count=1

lint:
	@test -z "$$(gofmt -l internal cmd main.go)" || (echo "gofmt needed"; gofmt -l internal cmd main.go; exit 1)
	go vet ./...

harness: | lab
	@printf 'provider_installation {\n  dev_overrides {\n    "simcop2387/lantronix-slcxx-o2" = "%s/dist"\n  }\n  direct {}\n}\n' "$$(pwd)" > $(DEVTFRC)
	@echo "wrote $(DEVTFRC) (dev_overrides -> $$(pwd)/dist)"

lab:
	mkdir -p lab

smoke: build harness
	TF_CLI_CONFIG_FILE=$(CURDIR)/$(DEVTFRC) $(TERRAFORM) -chdir=examples plan | tail -5
	TF_CLI_CONFIG_FILE=$(CURDIR)/$(DEVTFRC) $(TOFU) -chdir=examples plan | tail -5

acc-tf: build harness
	TF_ACC=1 TF_CLI_CONFIG_FILE=$(CURDIR)/$(DEVTFRC) TF_ACC_TERRAFORM_PATH=$(TERRAFORM) \
		go test ./internal/provider/ -timeout 90m -v -run TestAcc

acc-tofu: build harness
	TF_ACC=1 TF_CLI_CONFIG_FILE=$(CURDIR)/$(DEVTFRC) TF_ACC_TERRAFORM_PATH=$(TOFU) \
		go test ./internal/provider/ -timeout 90m -v -run TestAcc

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest

tidy:
	go mod tidy
