COMMIT_ID := $(shell git rev-parse --short=8 HEAD)
COMMIT_DATE := $(shell git show -s --format=%cd --date=short HEAD)
VERSION_FILE := _version.txt
VERSION_BASE := $(COMMIT_DATE)_$(COMMIT_ID)
VERSION_NUMBER := $(shell cat ${VERSION_FILE})
VERSION_PUBLIC := $(VERSION_NUMBER)
PACKAGE_EXTERNAL := perfspect_$(VERSION_NUMBER).tgz
BINARY_FINAL := perfspect
BINARY_COLLECT := perf-collect
BINARY_POSTPROCESS := perf-postprocess
default: all

.PHONY: all test default dist clean format format_check security_scan flakes source_check checkmake dist/version_file dist/$(SOURCE_PACKAGE)

clean_dir:
	rm -rf build/*
	rm -rf dist/*
	#sudo rm -rf test/$(BINARY_FINAL)
	rm -rf src/__pycache__

build_dir: clean_dir
	mkdir -p build

build/pmu-checker:
	cd pmu-checker && make
	cp pmu-checker/pmu-checker build/
	strip -s -p --strip-unneeded build/pmu-checker

build/libtsc:
	gcc -fno-strict-overflow -fno-delete-null-pointer-checks -fwrapv -fPIC -shared -o src/libtsc.so src/calibrate.c

build-public/collect:
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
					--add-data "./events/spr.txt:." \
					--add-data "./events/icx_aws.txt:." \
					--add-data "./events/clx_aws.txt:." \
					--add-data "./events/skx_aws.txt:." \
					--add-binary "../build/pmu-checker:." \
					--runtime-tmpdir . \
					--exclude-module readline
	
	cp $(TMPDIR)/dist/$(BINARY_COLLECT) build/
	rm -rf $(TMPDIR)

build-public/postprocess:
	$(eval TMPDIR := $(shell mktemp -d build.XXXXXX))
	mkdir -p $(TMPDIR)/src
	mkdir -p $(TMPDIR)/events
	cp src/* $(TMPDIR)/src && cp events/* $(TMPDIR)/events && cp *.py $(TMPDIR)
	sed -i 's/PerfSpect_DEV_VERSION/$(VERSION_PUBLIC)/g' $(TMPDIR)/src/perf_helpers.py 
	cd $(TMPDIR) && pyinstaller -F perf-postprocess.py -n perf-postprocess \
					--add-data "./events/metric_skx_clx.json:." \
					--add-data "./events/metric_bdx.json:." \
					--add-data "./events/metric_icx.json:." \
					--add-data "./events/metric_spr.json:." \
					--add-data "./events/metric_icx_aws.json:." \
					--runtime-tmpdir . \
					--exclude-module readline
	cp $(TMPDIR)/dist/perf-postprocess build/
	rm -rf $(TMPDIR)

dist/$(PACKAGE_EXTERNAL): build_dir build/pmu-checker build/libtsc build-public/collect build-public/postprocess
	rm -rf dist/$(BINARY_FINAL)/
	mkdir -p dist/$(BINARY_FINAL)
	cp build/$(BINARY_COLLECT) dist/$(BINARY_FINAL)/$(BINARY_COLLECT)
	cp build/$(BINARY_POSTPROCESS) dist/$(BINARY_FINAL)/$(BINARY_POSTPROCESS)
	cp LICENSE dist/$(BINARY_FINAL)/
	cp README.md dist/$(BINARY_FINAL)/README.md
	cd dist && tar -czf $(PACKAGE_EXTERNAL) $(BINARY_FINAL)
	cd dist && cp -r $(BINARY_FINAL) ../build/  
	rm -rf dist/$(BINARY_FINAL)/
	cd dist && md5sum $(PACKAGE_EXTERNAL) > $(PACKAGE_EXTERNAL).md5

test:
	cd dist && tar -xvf perfspect_$(VERSION_PUBLIC).tgz && cp -r $(BINARY_FINAL) ../test/.
	cd test && pytest

format:
	black src
	black *.py

format_check:
	black --check src
	black --check perf-collect.py perf-postprocess.py

error_check: # ignore false positives
	flake8 --ignore=E501,W503,F403,F405,E741 src
	flake8 --ignore=E203,E501,E722,W503,F403,F405 *.py --exclude simpleeval.py,perfmon.py,average.py

source_check: security_scan format_check error_check

dist: source_check dist/$(PACKAGE_EXTERNAL) 
