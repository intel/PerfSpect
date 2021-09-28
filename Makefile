VERSION_FILE := _version.txt
VERSION_NUMBER := $(shell cat ${VERSION_FILE})
VERSION_PUBLIC := $(VERSION_NUMBER)
PACKAGE := perfspect_$(VERSION_NUMBER).tgz
BINARY_FINAL := perfspect
BINARY_COLLECT := perf-collect
BINARY_POSTPROCESS := perf-postprocess

default: all

.PHONY: all test default dist clean format format_check security_scan flakes mypy pytype source_check checkmake dist/$(PACKAGE)

clean_dir:
	rm -rf build/*
	rm -rf dist/*
	sudo rm -rf test/perf*
	rm -rf src/__pycache__

build_dir: clean_dir
	mkdir -p build
	mkdir -p dist

build/pmu-checker:
	cd pmu-checker && make
	cp pmu-checker/pmu-checker build/
	strip -s -p --strip-unneeded build/pmu-checker

build/libtsc:
	cd src && gcc -shared -o libtsc.so -fPIC calibrate.c

build/collect:
	$(eval TMPDIR := $(shell mktemp -d build.XXXXXX))
	mkdir -p $(TMPDIR)/src
	mkdir -p $(TMPDIR)/events
	cp src/* $(TMPDIR)/src && cp events/* $(TMPDIR)/events && cp *.py $(TMPDIR)
	sed -i 's/PerfSpect_DEV_VERSION/$(VERSION_PUBLIC)/g' $(TMPDIR)/src/perf_helpers.py
	cd $(TMPDIR) && pyinstaller -F perf-collect.py -n $(BINARY_COLLECT) \
					--add-data "./src/libtsc.so:." \
					--add-data "./events/bdx.txt:." \
					--add-data "./events/skx.txt:." \
					--add-data "./events/clx.txt:." \
					--add-data "./events/icx.txt:." \
					--add-binary "../build/pmu-checker:." \
					--runtime-tmpdir . 
	cp $(TMPDIR)/dist/$(BINARY_COLLECT) build/
	rm -rf $(TMPDIR)

build/postprocess:
	$(eval TMPDIR := $(shell mktemp -d build.XXXXXX))
	git clone https://github.com/danthedeckie/simpleeval.git
	cp simpleeval/simpleeval.py .
	mkdir -p $(TMPDIR)/src
	mkdir -p $(TMPDIR)/events
	cp src/* $(TMPDIR)/src && cp events/* $(TMPDIR)/events && cp *.py $(TMPDIR)
	sed -i 's/PerfSpect_DEV_VERSION/$(VERSION_PUBLIC)/g' $(TMPDIR)/src/perf_helpers.py
	cd $(TMPDIR) && pyinstaller -F perf-postprocess.py -n perf-postprocess \
					--add-data "./events/metric_skx_clx.json:." \
					--add-data "./events/metric_bdx.json:." \
					--add-data "./events/metric_icx.json:." \
					--add-data="simpleeval.py:." \
					--runtime-tmpdir .
	cp $(TMPDIR)/dist/perf-postprocess build/
	rm -rf simpleeval && rm -f simpleeval.py
	rm -rf $(TMPDIR)

dist/$(PACKAGE): build_dir build/pmu-checker build/libtsc build/collect build/postprocess
	rm -rf dist/*
	cp build/$(BINARY_COLLECT) dist/$(BINARY_COLLECT)
	cp build/$(BINARY_POSTPROCESS) dist/$(BINARY_POSTPROCESS)
	rm -rf build


test:
	cp dist/$(BINARY_COLLECT) dist/$(BINARY_POSTPROCESS) test/*
	cd test && pytest

security_scan: src/*.py
	bandit src
	bandit *.py

format:
	black src
	black *.py

format_check:
	black --check src
	black --check *.py

error_check: # ignore false positives
	flake8 --ignore=E501,W503,F403,F405 src
	flake8 --ignore=E203,E501,E722,W503,F403,F405 *.py

source_check: security_scan format_check error_check

dist: source_check dist/$(PACKAGE)

