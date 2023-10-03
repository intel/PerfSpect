COMMIT_ID := $(shell git rev-parse --short=8 HEAD)
COMMIT_DATE := $(shell git show -s --format=%cd --date=short HEAD)
VERSION_FILE := _version.txt
VERSION_BASE := $(COMMIT_DATE)_$(COMMIT_ID)
VERSION_NUMBER := $(shell cat ${VERSION_FILE})
VERSION_PUBLIC := $(VERSION_NUMBER)
PACKAGE_EXTERNAL := perfspect.tgz
BINARY_FINAL := perfspect
BINARY_COLLECT := perf-collect
BINARY_POSTPROCESS := perf-postprocess
default: dist

.PHONY: test default dist format format_check style_error_check pytype check dist/version_file dist/$(SOURCE_PACKAGE)

clean_dir:
	rm -rf build/*
	rm -rf dist/*
	#sudo rm -rf test/$(BINARY_FINAL)
	rm -rf src/__pycache__

build_dir: clean_dir
	mkdir -p build

build/libtsc:
	gcc -fno-strict-overflow -fno-delete-null-pointer-checks -fwrapv -fPIC -shared -o src/libtsc.so src/calibrate.c

build-public/collect:
	$(eval TMPDIR := $(shell mktemp -d build.XXXXXX))
	mkdir -p $(TMPDIR)/src
	mkdir -p $(TMPDIR)/events
	cp src/* $(TMPDIR)/src && cp events/* $(TMPDIR)/events && cp *.py $(TMPDIR)
	sed -i 's/PerfSpect_DEV_VERSION/$(VERSION_PUBLIC)/g' $(TMPDIR)/src/perf_helpers.py
	cp perf-collect.spec $(TMPDIR)
	cd $(TMPDIR) && pyinstaller perf-collect.spec
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
					--add-data "./events/metric_srf.json:." \
					--add-data "./src/base.html:." \
					--runtime-tmpdir . \
					--exclude-module readline
					--bootloader-ignore-signals
	cp $(TMPDIR)/dist/perf-postprocess build/
	rm -rf $(TMPDIR)

dist/$(PACKAGE_EXTERNAL): build_dir build/libtsc build-public/collect build-public/postprocess
	rm -rf dist/$(BINARY_FINAL)/
	mkdir -p dist/$(BINARY_FINAL)
	cp build/$(BINARY_COLLECT) dist/$(BINARY_FINAL)/$(BINARY_COLLECT)
	cp build/$(BINARY_POSTPROCESS) dist/$(BINARY_FINAL)/$(BINARY_POSTPROCESS)
	cp LICENSE dist/$(BINARY_FINAL)/
	cd dist && tar -czf $(PACKAGE_EXTERNAL) $(BINARY_FINAL)
	cd dist && cp -r $(BINARY_FINAL) ../build/  
	rm -rf dist/$(BINARY_FINAL)/
	cd dist && md5sum $(PACKAGE_EXTERNAL) > $(PACKAGE_EXTERNAL).md5

test:
	cd dist && tar -xvf perfspect.tgz && cp -r $(BINARY_FINAL) ../test/.
	cd test && pytest

format_check:
	black --check *.py src

format:
	black *.py src

style_error_check:
	# ignore long lines and conflicts with black, i.e., black wins
	flake8 *.py src --ignore=E501,W503,E203

pytype: *.py src/*.py
	pytype ./*.py

check: format_check style_error_check pytype

dist: check dist/$(PACKAGE_EXTERNAL) 
