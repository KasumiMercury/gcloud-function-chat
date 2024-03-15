# Load the .env file if it exists
ifneq (,$(wildcard ./.env))
	include .env
	export
endif

launch:
	FUNCTION_TARGET=$(ENTRY_POINT) go run cmd/main.go

deploy:
ifndef SERVICE_NAME
	$(deploy-usage)
endif
ifndef ENTRY_POINT
	$(deploy-usage)
endif
	gcloud functions deploy $(SERVICE_NAME) \
		--gen2 \
		--runtime go121 \
		--region=asia-northeast1 \
		--source . \
		--entry-point=$(ENTRY_POINT) \
		--trigger-http
define deploy-usage
	@echo "Not enough parameters"
	@exit 1
endef
