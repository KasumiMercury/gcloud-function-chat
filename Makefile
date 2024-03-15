# Load the .env file if it exists
ifneq (,$(wildcard ./.env))
	include .env
	export
endif

launch:
	FUNCTION_TARGET=$(ENTRY_POINT) go run cmd/main.go

deploy:
# Check if the required parameters are set
ifndef SERVICE_NAME
	$(deploy-usage)
endif
ifndef ENTRY_POINT
	$(deploy-usage)
endif
ifndef YOUTUBE_API_KEY
	$(deploy-usage)
endif
ifndef DSN
	$(deploy-usage)
endif
ifndef TARGET_CHANNEL_ID
	$(deploy-usage)
endif
ifndef STATIC_TARGET
	$(deploy-usage)
endif
# Deploy the function
	gcloud functions deploy $(SERVICE_NAME) \
		--no-gen2 \
		--runtime go121 \
		--region=asia-northeast1 \
		--source . \
		--entry-point=$(ENTRY_POINT) \
		--trigger-http
define deploy-usage
	@echo "Not enough parameters"
	@exit 1
endef
