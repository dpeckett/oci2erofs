# Overlayfs Test Data

```bash
docker build -t overlayfs-testdata .
skopeo copy docker-daemon:overlayfs-testdata:latest oci:./image:latest
```