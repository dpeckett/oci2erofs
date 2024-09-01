# oci2erofs

Convert OCI images into [EROFS](https://erofs.docs.kernel.org/en/latest/) filesystems
completely in userspace.

## Installation

### From APT

Add my apt repository to your system:

*Currently packages are only published for Debian 12 (Bookworm).*

```shell
curl -fsL https://apt.pecke.tt/signing_key.asc | sudo tee /etc/apt/keyrings/apt-pecke-tt-keyring.asc > /dev/null
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/apt-pecke-tt-keyring.asc] http://apt.pecke.tt $(. /etc/os-release && echo $VERSION_CODENAME) stable" | sudo tee /etc/apt/sources.list.d/apt-pecke-tt.list > /dev/null
```

Then install oci2erofs:

```shell
sudo apt update
sudo apt install oci2erofs
```

### GitHub Releases

Download statically linked binaries from the GitHub releases page: 

[Latest Release](https://github.com/dpeckett/oci2erofs/releases/latest)

## Usage

To create an EROFS image from a directory containing an OCI image:

```shell
oci2erofs -o image.erofs ./oci-image
```

oci2erofs also supports converting an OCI image from compressed tarballs:

```shell
oci2erofs -o image.erofs ./oci-image.tar
```

## Limitations

- No support for compression or extended attributes.