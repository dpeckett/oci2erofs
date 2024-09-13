# oci2erofs

Convert OCI images into [EROFS](https://erofs.docs.kernel.org/en/latest/) filesystems.

## Installation

### From APT

Add my [apt repository](https://github.com/dpeckett/apt.dpeckett.dev?tab=readme-ov-file#usage) to your system.

Then install oci2erofs:

*Currently packages are only published for Debian 12 (Bookworm).*

```shell
sudo apt update
sudo apt install oci2erofs
```

### GitHub Releases

Download statically linked binaries from the GitHub releases page: 

[Latest Release](https://github.com/immutos/oci2erofs/releases/latest)

## Usage

To create an EROFS image from a directory containing an OCI image:

```shell
oci2erofs -o image.erofs ./oci-image
```

Tarballs are also supported:

```shell
oci2erofs -o image.erofs ./oci-image.tar
```

## Telemetry

By default oci2erofs gathers anonymous crash and usage statistics. This anonymized
data is processed on our servers within the EU and is not shared with third
parties. You can opt out of telemetry by setting the `DO_NOT_TRACK=1`
environment variable.

## Limitations

- No support for compression or extended attributes.