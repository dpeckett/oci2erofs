VERSION 0.8
FROM golang:1.22-bookworm
WORKDIR /workspace

all:
  ARG VERSION=dev
  COPY (+build/oci2erofs --GOARCH=amd64) ./dist/oci2erofs-linux-amd64
  COPY (+build/oci2erofs --GOARCH=arm64) ./dist/oci2erofs-linux-arm64
  COPY (+build/oci2erofs --GOARCH=riscv64) ./dist/oci2erofs-linux-riscv64
  COPY (+build/oci2erofs --GOOS=darwin --GOARCH=amd64) ./dist/oci2erofs-darwin-amd64
  COPY (+build/oci2erofs --GOOS=darwin --GOARCH=arm64) ./dist/oci2erofs-darwin-arm64
  COPY (+build/oci2erofs --GOOS=windows --GOARCH=amd64) ./dist/oci2erofs-windows-amd64.exe
  COPY (+package/*.deb --GOARCH=amd64) ./dist/
  COPY (+package/*.deb --GOARCH=arm64) ./dist/
  COPY (+package/*.deb --GOARCH=riscv64) ./dist/
  RUN cd dist && find . -type f | sort | xargs sha256sum >> ../sha256sums.txt
  SAVE ARTIFACT ./dist/* AS LOCAL dist/
  SAVE ARTIFACT ./sha256sums.txt AS LOCAL dist/sha256sums.txt

build:
  ARG GOOS=linux
  ARG GOARCH=amd64
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  ARG VERSION=dev
  RUN CGO_ENABLED=0 go build --ldflags "-s -X 'github.com/dpeckett/oci2erofs/internal/constants.Version=${VERSION}'" -o oci2erofs main.go
  SAVE ARTIFACT ./oci2erofs AS LOCAL dist/oci2erofs-${GOOS}-${GOARCH}

tidy:
  LOCALLY
  ENV GOTOOLCHAIN=go1.22.1
  RUN go mod tidy
  RUN go fmt ./...

lint:
  FROM golangci/golangci-lint:v1.59.1
  WORKDIR /workspace
  COPY . ./
  RUN golangci-lint run --timeout 5m ./...

test:
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN go test -coverprofile=coverage.out -v ./...
  SAVE ARTIFACT ./coverage.out AS LOCAL coverage.out
  # A quick integration test to ensure that things are working.
  RUN go build .
  RUN ./oci2erofs -o toybox.erofs testdata/toybox.tar
  RUN --privileged mount -t erofs ./toybox.erofs /mnt \
    && test -f /mnt/usr/bin/toybox \
    && umount /mnt

package:
  FROM debian:bookworm
  # Use bookworm-backports for newer golang versions
  RUN echo "deb http://deb.debian.org/debian bookworm-backports main" > /etc/apt/sources.list.d/backports.list
  RUN apt update
  # Tooling
  RUN apt install -y git curl devscripts dpkg-dev debhelper-compat git-buildpackage libfaketime dh-sequence-golang \
    golang-any=2:1.22~3~bpo12+1 golang-go=2:1.22~3~bpo12+1 golang-src=2:1.22~3~bpo12+1 \
    gcc-aarch64-linux-gnu gcc-riscv64-linux-gnu
  RUN curl -fsL -o /etc/apt/keyrings/apt-pecke-tt-keyring.asc https://apt.pecke.tt/signing_key.asc \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/apt-pecke-tt-keyring.asc] http://apt.pecke.tt $(. /etc/os-release && echo $VERSION_CODENAME) stable" > /etc/apt/sources.list.d/apt-pecke-tt.list \
    && apt update
  # Build Dependencies
  RUN apt install -y \
    golang-github-dpeckett-archivefs-dev \
    golang-github-dpeckett-telemetry-dev \
    golang-github-dpeckett-uncompr-dev \
    golang-github-opencontainers-image-spec-dev=1.1.0-2~bpo12+1 \
    golang-github-pierrec-lz4-dev=4.1.18-1~bpo12+1 \
    golang-github-rogpeppe-go-internal-dev \
    golang-github-stretchr-testify-dev \
    golang-github-urfave-cli-v2-dev
  RUN mkdir -p /workspace/oci2erofs
  WORKDIR /workspace/oci2erofs
  COPY . .
  RUN if [ -n "$(git status --porcelain)" ]; then echo "Please commit your changes."; exit 1; fi
  RUN if [ -z "$(git describe --tags --exact-match 2>/dev/null)" ]; then echo "Current commit is not tagged."; exit 1; fi
  COPY debian/scripts/generate-changelog.sh /usr/local/bin/generate-changelog.sh
  RUN chmod +x /usr/local/bin/generate-changelog.sh
  ENV DEBEMAIL="damian@pecke.tt"
  ENV DEBFULLNAME="Damian Peckett"
  RUN /usr/local/bin/generate-changelog.sh
  RUN VERSION=$(git describe --tags --abbrev=0 | tr -d 'v') \
    && tar -czf ../oci2erofs_${VERSION}.orig.tar.gz --exclude-vcs .
  ARG GOARCH
  RUN dpkg-buildpackage -d -us -uc --host-arch=${GOARCH}
  SAVE ARTIFACT /workspace/*.deb AS LOCAL dist/
