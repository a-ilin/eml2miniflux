MINIFLUX_VERSION := $(shell git -C sub/miniflux describe --tags --abbrev=0)
LD_FLAGS         := "-X 'main.MinifluxVersion=$(MINIFLUX_VERSION)'"

# source: https://stackoverflow.com/a/30225575
ifeq ($(OS),Windows_NT)
	BIN          := bin/eml2miniflux.exe

	mkdir = mkdir $(subst /,\,$(1)) > nul 2>&1 || (exit 0)
	rm = $(wordlist 2,65535,$(foreach FILE,$(subst /,\,$(1)),& del $(FILE) > nul 2>&1)) || (exit 0)
	rmdir = rmdir $(subst /,\,$(1)) > nul 2>&1 || (exit 0)
	echo = echo $(1)
else
	BIN          := bin/eml2miniflux

	mkdir = mkdir -p $(1)
	rm = rm $(1) > /dev/null 2>&1 || true
	rmdir = rmdir $(1) > /dev/null 2>&1 || true
	echo = echo "$(1)"
endif


.PHONY: \
	eml2miniflux \
	clean \
	test

eml2miniflux:
	@ $(call mkdir, bin)
	go build -buildmode=pie -ldflags=$(LD_FLAGS) -o $(BIN) .

clean:
	@ $(call rm, $(BIN))

test:
	go test -cover -race -count=1 ./...
