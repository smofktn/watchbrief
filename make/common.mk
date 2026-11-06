# Shared paths only; operational recipes belong to each Makefile.
PROJECT_ROOT := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/..)
ASDF_SHIMS := $(if $(ASDF_DATA_DIR),$(ASDF_DATA_DIR),$(HOME)/.asdf)/shims
export PATH := $(PROJECT_ROOT)/.local/bin:$(ASDF_SHIMS):$(HOME)/.local/bin:$(PATH)

GO ?= go
NPM ?= npm
PYTHON ?= python3
DOCKER ?= docker

.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
