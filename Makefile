include golang.mk
.DEFAULT_GOAL := test # override default goal set in library makefile

VERSION := $(shell cat VERSION)
SHELL := /bin/bash
PKGS := $(shell go list ./... | grep -v /vendor)

vendor: golang-godep-vendor-deps
		$(call golang-godep-vendor,$(PKGS))

test: $(PKGS)
$(PKGS): golang-test-all-strict-deps
		$(call golang-test-all-strict,$@)
