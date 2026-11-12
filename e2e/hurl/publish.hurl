# POST /{product}/artifacts/{version}/publish
# Checks the staged files of a version as a whole, then publishes them in one rename.
#
# The successful publishes are in seed/ (goreleaser dist, folder mode, latest or not): every
# other scenario reads what they published. This file covers the refusals only, so that it
# publishes nothing and the totals read by the other scenarios stay the same.
# A refused publish keeps the staged files: calling publish again gives the same answer.

# --- Authentication.
POST {{url}}/demo-publish/artifacts/v1.0.0/publish
HTTP 401
[Asserts]
header "WWW-Authenticate" == "Bearer"
jsonpath "$.error" == "invalid or missing bearer token"

POST {{url}}/demo-publish/artifacts/v1.0.0/publish
Authorization: Bearer wrong-token
HTTP 401

# --- Request.
POST {{url}}/products/artifacts/v1.0.0/publish
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "product name products is reserved"

POST {{url}}/demo-publish/artifacts/latest/publish
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "invalid product or version name"

POST {{url}}/demo-publish/artifacts/release.json/publish
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "version name release.json is reserved"

POST {{url}}/demo-publish/artifacts/v1.0.0/publish
Authorization: Bearer {{token}}
[Query]
latest: maybe
HTTP 400
[Asserts]
jsonpath "$.error" == "latest must be true or false"

POST {{url}}/demo-publish/artifacts/v0.0.0/publish
Authorization: Bearer {{token}}
HTTP 404
[Asserts]
jsonpath "$.error" == "nothing uploaded for demo-publish/v0.0.0"

# --- A published version is immutable.
POST {{url}}/gitlab/artifacts/v0.1.0/publish
Authorization: Bearer {{token}}
[Query]
latest: true
HTTP 409
[Asserts]
jsonpath "$.error" == "version gitlab/v0.1.0 is already published and cannot be changed"

# --- registry.yaml is required.
PUT {{url}}/demo-publish/artifacts/v1.0.0-noreg/files/demo_1.0.0.tar.gz
Authorization: Bearer {{token}}
```
demo
```
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-noreg/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "registry.yaml not uploaded: every version needs its product card"

# Staging is kept: the same answer again, not 404.
POST {{url}}/demo-publish/artifacts/v1.0.0-noreg/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "registry.yaml not uploaded: every version needs its product card"

# --- At least one artifact besides registry.yaml.
PUT {{url}}/demo-publish/artifacts/v1.0.0-noart/files/registry.yaml
Authorization: Bearer {{token}}
file,fixtures/legacy-tool/v0.9.0/registry.yaml;
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-noart/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "no artifact to publish: upload at least one file besides registry.yaml"

# --- An uploaded checksums file must list every file, with its sha256.
PUT {{url}}/demo-publish/artifacts/v1.0.0-sums/files/registry.yaml
Authorization: Bearer {{token}}
file,fixtures/legacy-tool/v0.9.0/registry.yaml;
HTTP 201

PUT {{url}}/demo-publish/artifacts/v1.0.0-sums/files/demo_1.0.0_linux_amd64.tar.gz
Authorization: Bearer {{token}}
```
linux
```
HTTP 201

PUT {{url}}/demo-publish/artifacts/v1.0.0-sums/files/demo_1.0.0_checksums.txt
Authorization: Bearer {{token}}
```
not a checksum line
```
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-sums/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "demo_1.0.0_checksums.txt: line 1 is not \"<sha256>  <name>\""

PUT {{url}}/demo-publish/artifacts/v1.0.0-sums/files/demo_1.0.0_checksums.txt
Authorization: Bearer {{token}}
```
0000000000000000000000000000000000000000000000000000000000000000  demo_1.0.0_linux_amd64.tar.gz
```
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-sums/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "demo_1.0.0_linux_amd64.tar.gz: sha256 is d745fba1cb70ab9dc02a80eeba8a1864a0f32b2941e008c0af389be7b56ba830 but demo_1.0.0_checksums.txt says 0000000000000000000000000000000000000000000000000000000000000000"

PUT {{url}}/demo-publish/artifacts/v1.0.0-sums/files/demo_1.0.0_darwin_arm64.tar.gz
Authorization: Bearer {{token}}
```
darwin
```
HTTP 201

PUT {{url}}/demo-publish/artifacts/v1.0.0-sums/files/demo_1.0.0_checksums.txt
Authorization: Bearer {{token}}
```
d745fba1cb70ab9dc02a80eeba8a1864a0f32b2941e008c0af389be7b56ba830  demo_1.0.0_linux_amd64.tar.gz
```
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-sums/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "demo_1.0.0_darwin_arm64.tar.gz is not listed in demo_1.0.0_checksums.txt"

# A second checksums file.
PUT {{url}}/demo-publish/artifacts/v1.0.0-sums/files/checksums.txt
Authorization: Bearer {{token}}
```
d745fba1cb70ab9dc02a80eeba8a1864a0f32b2941e008c0af389be7b56ba830  demo_1.0.0_linux_amd64.tar.gz
```
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-sums/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" startsWith "several checksums files: "

# --- metadata.json must describe the version published.
PUT {{url}}/demo-publish/artifacts/v1.0.0-meta/files/registry.yaml
Authorization: Bearer {{token}}
file,fixtures/legacy-tool/v0.9.0/registry.yaml;
HTTP 201

PUT {{url}}/demo-publish/artifacts/v1.0.0-meta/files/demo_1.0.0.tar.gz
Authorization: Bearer {{token}}
```
demo
```
HTTP 201

PUT {{url}}/demo-publish/artifacts/v1.0.0-meta/files/metadata.json
Authorization: Bearer {{token}}
```
{"project_name": "demo", "tag": "v9.9.9", "version": "9.9.9"}
```
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-meta/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "metadata.json describes v9.9.9 (version 9.9.9), not v1.0.0-meta"

PUT {{url}}/demo-publish/artifacts/v1.0.0-meta/files/metadata.json
Authorization: Bearer {{token}}
```
{"project_name": "demo"}
```
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-meta/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "metadata.json has neither tag nor version"

# --- With artifacts.json, every uploaded file must be listed in it.
PUT {{url}}/demo-publish/artifacts/v1.0.0-arts/files/registry.yaml
Authorization: Bearer {{token}}
file,fixtures/legacy-tool/v0.9.0/registry.yaml;
HTTP 201

PUT {{url}}/demo-publish/artifacts/v1.0.0-arts/files/demo_1.0.0.tar.gz
Authorization: Bearer {{token}}
```
demo
```
HTTP 201

PUT {{url}}/demo-publish/artifacts/v1.0.0-arts/files/stray.txt
Authorization: Bearer {{token}}
file,fixtures/invalid/stray.txt;
HTTP 201

PUT {{url}}/demo-publish/artifacts/v1.0.0-arts/files/artifacts.json
Authorization: Bearer {{token}}
```
[{"name": "demo_1.0.0.tar.gz", "path": "dist/demo_1.0.0.tar.gz", "type": "Archive", "extra": {"Format": "tar.gz"}}]
```
HTTP 201

POST {{url}}/demo-publish/artifacts/v1.0.0-arts/publish
Authorization: Bearer {{token}}
HTTP 422
[Asserts]
jsonpath "$.error" == "stray.txt is uploaded but artifacts.json does not list it"

# Nothing above was published.
GET {{url}}/demo-publish/index
HTTP 404
