# GET /{product}/artifacts/{version}/files/{name}
# One file of a published version. Staged files are never served. Errors are 404 in plain text.

GET {{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz
HTTP 200
[Captures]
last_modified: header "Last-Modified"
[Asserts]
header "Content-Type" == "application/octet-stream"
header "Content-Disposition" == "attachment; filename=gitlab_0.1.0_linux_amd64.tar.gz"
header "Content-Length" == "26"
header "Accept-Ranges" == "bytes"
header "Last-Modified" exists
sha256 == hex,5e3c935149d6200dc6a2796e4839e9b6c0a4fd05368b83be59ba4e461edd7683;

# Resume a download.
GET {{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz
Range: bytes=7-
HTTP 206
[Asserts]
header "Content-Range" == "bytes 7-25/26"
body == "linux amd64 v0.1.0\n"

# Conditional download.
GET {{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz
If-Modified-Since: {{last_modified}}
HTTP 304

# The documents written by publish are files of the version too.
GET {{url}}/gitlab/artifacts/v0.1.0/files/release.json
HTTP 200
[Asserts]
bytes count > 0

GET {{url}}/gitlab/artifacts/v0.1.0/files/artifacts.goreleaser.json
HTTP 200
[Asserts]
body contains "dist/gitlab_0.1.0_linux_amd64.tar.gz"

GET {{url}}/billing-db-migrations/artifacts/v1.0.0/files/checksums.txt
HTTP 200
[Asserts]
body == "cde6915139e6cf8cf66b69977ae8b8b3bb6b5ed911f87faa802982f12dad9a21  billing-db-migrations_1.0.0.tar.gz\n"

# "latest" is not a version: resolve it with GET /{product}/releases/latest.
GET {{url}}/gitlab/artifacts/latest/files/registry.yaml
HTTP 404
[Asserts]
body == "404 page not found\n"

# A staged file is not public until publish.
PUT {{url}}/demo-download/artifacts/v1.0.0/files/staged.txt
Authorization: Bearer {{token}}
```
staged only
```
HTTP 201

GET {{url}}/demo-download/artifacts/v1.0.0/files/staged.txt
HTTP 404

# Missing file, unknown version, hidden name.
GET {{url}}/gitlab/artifacts/v0.1.0/files/missing.tar.gz
HTTP 404

GET {{url}}/gitlab/artifacts/v9.9.9/files/registry.yaml
HTTP 404

GET {{url}}/gitlab/artifacts/v0.1.0/files/.hidden
HTTP 404
