# PUT /{product}/artifacts/{version}/files/{name}
# Streams one file into the staging area of a version. Nothing is public before publish.
# Product demo-upload is only staged here, never published, so other scenarios do not see it.

# --- Authentication: a bearer token, checked before anything else.
PUT {{url}}/demo-upload/artifacts/v1.0.0/files/registry.yaml
file,fixtures/gitlab/v0.1.0/registry.yaml;
HTTP 401
[Asserts]
header "WWW-Authenticate" == "Bearer"
jsonpath "$.error" == "invalid or missing bearer token"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/registry.yaml
Authorization: Bearer wrong-token
file,fixtures/gitlab/v0.1.0/registry.yaml;
HTTP 401
[Asserts]
header "WWW-Authenticate" == "Bearer"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/registry.yaml
Authorization: Basic {{token}}
file,fixtures/gitlab/v0.1.0/registry.yaml;
HTTP 401

# Authentication comes first: even an invalid name answers 401 without a token.
PUT {{url}}/products/artifacts/v1.0.0/files/registry.yaml
HTTP 401

# --- Accepted: 201 with the name, size and sha256 received.
PUT {{url}}/demo-upload/artifacts/v1.0.0/files/registry.yaml
Authorization: Bearer {{token}}
X-Checksum-Sha256: ECEFEE6EA38F8BE32378808D0D88C28F7E97C81B1AF88252CE61975D5046CCA7
file,fixtures/gitlab/v0.1.0/registry.yaml;
HTTP 201
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.name" == "registry.yaml"
jsonpath "$.size" == 378
jsonpath "$.sha256" == "ecefee6ea38f8be32378808d0d88c28f7e97c81b1af88252ce61975d5046cca7"

# Uploading the same name again replaces the staged file.
PUT {{url}}/demo-upload/artifacts/v1.0.0/files/demo_1.0.0.tar.gz
Authorization: Bearer {{token}}
```
first
```
HTTP 201

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/demo_1.0.0.tar.gz
Authorization: Bearer {{token}}
```
second
```
HTTP 201
[Asserts]
jsonpath "$.size" == 7
jsonpath "$.sha256" == "480c2336b410f1ad5f8bf1b28944490255804b65350c527787e74ebdd511e3a4"

# --- Names.
PUT {{url}}/products/artifacts/v1.0.0/files/registry.yaml
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "product name products is reserved"

PUT {{url}}/demo-upload/artifacts/latest/files/registry.yaml
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "invalid product or version name"

PUT {{url}}/demo%20upload/artifacts/v1.0.0/files/registry.yaml
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "invalid product or version name"

PUT {{url}}/demo-upload/artifacts/index.json/files/registry.yaml
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "version name index.json is reserved"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/my%20file.txt
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "invalid file name: use [A-Za-z0-9._-] only"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/release.json
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "release.json is reserved: publishing writes it"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/index.json
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "index.json is reserved: publishing writes it"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/artifacts.goreleaser.json
Authorization: Bearer {{token}}
HTTP 400
[Asserts]
jsonpath "$.error" == "artifacts.goreleaser.json is reserved: publishing writes it"

# --- Content checks: nothing is kept when they fail.
PUT {{url}}/demo-upload/artifacts/v1.0.0/files/demo_1.0.0.zip
Authorization: Bearer {{token}}
X-Checksum-Sha256: 0000000000000000000000000000000000000000000000000000000000000000
```
zip
```
HTTP 422
[Asserts]
jsonpath "$.error" startsWith "sha256 mismatch: got "
jsonpath "$.error" endsWith ", X-Checksum-Sha256 says 0000000000000000000000000000000000000000000000000000000000000000"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/registry.yaml
Authorization: Bearer {{token}}
file,fixtures/invalid/registry-bad-version.yaml;
HTTP 422
[Asserts]
jsonpath "$.error" == "registry.yaml: apiVersion must be local-registry/v1"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/registry.yaml
Authorization: Bearer {{token}}
file,fixtures/invalid/registry-unknown-key.yaml;
HTTP 422
[Asserts]
jsonpath "$.error" contains "field changes not found"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/registry.yaml
Authorization: Bearer {{token}}
```
apiVersion: local-registry/v1
owners:
  - email: nobody
```
HTTP 422
[Asserts]
jsonpath "$.error" == "registry.yaml: owners[0] has no name"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/CHANGELOG.md
Authorization: Bearer {{token}}
hex,fffe00;
HTTP 422
[Asserts]
jsonpath "$.error" == "CHANGELOG.md is not UTF-8 text"

# artifacts.json goes last: every file it publishes must already be staged.
PUT {{url}}/demo-upload/artifacts/v1.0.0/files/artifacts.json
Authorization: Bearer {{token}}
```
[{"name": "not-staged.tar.gz", "path": "dist/not-staged.tar.gz", "type": "Archive", "extra": {"Format": "tar.gz"}}]
```
HTTP 422
[Asserts]
jsonpath "$.error" contains "not-staged.tar.gz"

PUT {{url}}/demo-upload/artifacts/v1.0.0/files/artifacts.json
Authorization: Bearer {{token}}
```
[{"name": "demo_1.0.0.tar.gz", "path": "dist/demo_1.0.0.tar.gz", "type": "Archive", "extra": {"Format": "tar.gz"}}]
```
HTTP 201
[Asserts]
jsonpath "$.name" == "artifacts.json"

# The staged files stay private.
GET {{url}}/demo-upload/artifacts/v1.0.0/files/registry.yaml
HTTP 404

GET {{url}}/demo-upload/index
HTTP 404

# --- A published version is immutable.
PUT {{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz
Authorization: Bearer {{token}}
```
tampered
```
HTTP 409
[Asserts]
jsonpath "$.error" == "version gitlab/v0.1.0 is already published and cannot be changed"
