[group('check')]
test:
    gotestsum -- ./...

[group('check')]
lint:
    golangci-lint run ./...

[group('check')]
lint-fix:
    golangci-lint run ./... --fix

# Fuzz Extract for 60 seconds.
[group('check')]
fuzz:
    gotestsum -- ./extractor -run '^$' -fuzz FuzzExtract -fuzztime 60s

# Rebuild the local development server. Not part of the published package.
[group('server')]
dev:
    air

# Score the labelled pages and save the run. Reads data-text/, which is not in the repository.
[group('eval')]
eval:
    go run ./cmd/eval

# Append unlabelled phrases in data-text/ as "?". data-text/ is not in the repository.
[group('eval')]
eval-draft:
    go run ./cmd/eval -draft

# Compare the current data-text/ run with a saved JSON file. data-text/ is not in the repository.
[group('eval')]
eval-diff baseline:
    go run ./cmd/eval -baseline {{ baseline }}

# Report chrome left in the extracted pages. Reads data-text/, which is not in the repository.
[group('eval')]
cleancheck:
    go run ./cmd/cleancheck

# Fetch, check, and draft labels for the pages listed in urls. data-text/ is not in the repository.
[group('eval')]
pages urls *args:
    sh scripts/add-pages.sh {{ urls }} {{ args }}

# Download the treebanks into postag/models/ud/. The download is a local cache.
[group('models')]
pos-fetch:
    sh postag/models/fetch.sh

# Retrain the part-of-speech models. The committed files are lang/<code>/model.bin.gz.
[group('models')]
pos-train:
    sh postag/models/train.sh

# Download the Leipzig sentences into rarity/leipzig/. The download is a local cache.
[group('models')]
rarity-fetch:
    sh rarity/fetch.sh

# Rebuild the rarity tables. The committed files are lang/<code>/rarity.bin.gz.
[group('models')]
rarity-build:
    sh rarity/build.sh

# Build the browser extractor into the documentation site.
[group('site')]
site-build:
    mkdir -p site/public/assets site/public/js/lib
    GOOS=js GOARCH=wasm go build -o site/public/assets/extract.wasm ./cmd/sitewasm
    root="$(go env GOROOT)"; if [ -f "$root/lib/wasm/wasm_exec.js" ]; then src="$root/lib/wasm/wasm_exec.js"; else src="$root/misc/wasm/wasm_exec.js"; fi; cp -f "$src" site/public/js/lib/wasm_exec.js && chmod u+w site/public/js/lib/wasm_exec.js

# Build the browser extractor and serve the documentation site.
[group('site')]
site: site-build
    cd site && yarn dev

# Check the URL-text fixture and the host allowlist. Does not call Firecrawl.
[group('site')]
site-test:
    cd site && yarn test
