# POST /releases/{product}/{version}
# Every file of a published version with its download URL (format kept for existing clients).

POST {{url}}/releases/gitlab/v0.1.0
HTTP 200
[Captures]
linux_id: jsonpath "$.assets[4].id"
linux_url: jsonpath "$.assets[4].url"
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.tag_name" == "v0.1.0"
# Every file of the version, sorted by name, the files written by publish included.
jsonpath "$.assets" count == 8
jsonpath "$.assets[0].name" == "artifacts.goreleaser.json"
jsonpath "$.assets[1].name" == "artifacts.json"
jsonpath "$.assets[2].name" == "gitlab_0.1.0_checksums.txt"
jsonpath "$.assets[2].is_checksum" == true
jsonpath "$.assets[4].name" == "gitlab_0.1.0_linux_amd64.tar.gz"
jsonpath "$.assets[4].is_checksum" == false
jsonpath "$.assets[4].url" == "{{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz"
jsonpath "$.assets[4].id" matches /^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/
jsonpath "$.assets[7].name" == "release.json"

# The ids are stable: derived from product, version and name (UUID v5).
POST {{url}}/releases/gitlab/v0.1.0
HTTP 200
[Asserts]
jsonpath "$.assets[4].id" == "{{linux_id}}"

GET {{linux_url}}
HTTP 200
[Asserts]
sha256 == hex,5e3c935149d6200dc6a2796e4839e9b6c0a4fd05368b83be59ba4e461edd7683;

# "latest" resolves to the latest version.
POST {{url}}/releases/gitlab/latest
HTTP 200
[Asserts]
jsonpath "$.tag_name" == "v0.1.0"

# Folder mode: the generated checksums.txt is the checksum file.
POST {{url}}/releases/billing-db-migrations/v1.0.0
HTTP 200
[Asserts]
jsonpath "$.tag_name" == "v1.0.0"
jsonpath "$.assets[?(@.name == 'checksums.txt')].is_checksum" == true
jsonpath "$.assets[?(@.name == 'CHANGELOG.md')].is_checksum" == false

POST {{url}}/releases/gitlab/v9.9.9
HTTP 404
[Asserts]
jsonpath "$.error" == "not found"

POST {{url}}/releases/missing/v0.1.0
HTTP 404

# Read-only route: only POST is routed.
GET {{url}}/releases/gitlab/v0.1.0
HTTP 405
