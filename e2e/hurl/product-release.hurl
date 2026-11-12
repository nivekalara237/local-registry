# GET /{product}/releases/{version}
# release.json of a version: the files to download, with URL, size and sha256.

# A version published from a goreleaser dist.
GET {{url}}/gitlab/releases/v0.1.0
HTTP 200
[Captures]
artifact_url: jsonpath "$.artifacts[0].url"
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.apiVersion" == "local-registry/v1"
jsonpath "$.product" == "gitlab"
jsonpath "$.version" == "v0.1.0"
jsonpath "$.tag" == "v0.1.0"
jsonpath "$.commit" == "9c9d7664ceef"
jsonpath "$.prerelease" == false
jsonpath "$.deprecated" == false
jsonpath "$.publishedAt" isIsoDate
jsonpath "$.publishedBy" matches /^[0-9a-f]{12}$/
jsonpath "$.artifacts" count == 2
jsonpath "$.artifacts[0].name" == "gitlab_0.1.0_linux_amd64.tar.gz"
jsonpath "$.artifacts[0].kind" == "archive"
jsonpath "$.artifacts[0].format" == "tar.gz"
jsonpath "$.artifacts[0].mediaType" == "application/gzip"
jsonpath "$.artifacts[0].os" == "linux"
jsonpath "$.artifacts[0].arch" == "amd64"
jsonpath "$.artifacts[0].size" == 26
jsonpath "$.artifacts[0].sha256" == "5e3c935149d6200dc6a2796e4839e9b6c0a4fd05368b83be59ba4e461edd7683"
jsonpath "$.artifacts[0].url" == "{{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz"
jsonpath "$.checksums.name" == "gitlab_0.1.0_checksums.txt"
jsonpath "$.checksums.url" == "{{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_checksums.txt"

# The URL given in release.json downloads the file it describes.
GET {{artifact_url}}
HTTP 200
[Asserts]
sha256 == hex,5e3c935149d6200dc6a2796e4839e9b6c0a4fd05368b83be59ba4e461edd7683;

# "latest" resolves to the version published with latest=true, not to the highest one.
GET {{url}}/gitlab/releases/latest
HTTP 200
[Asserts]
jsonpath "$.version" == "v0.1.0"

# A pre-release published without metadata.json: no tag, no commit.
GET {{url}}/gitlab/releases/v0.2.0-rc.1
HTTP 200
[Asserts]
jsonpath "$.prerelease" == true
jsonpath "$.tag" == null
jsonpath "$.commit" == null

# Folder mode: no artifacts.json, the release notes come from CHANGELOG.md, checksums.txt is generated.
GET {{url}}/billing-db-migrations/releases/v1.0.0
HTTP 200
[Asserts]
jsonpath "$.displayName" == "Billing DB migrations"
jsonpath "$.changes" count == 1
jsonpath "$.changes[0].kind" == "added"
jsonpath "$.changes[0].description" == "Table orders."
jsonpath "$.artifacts" count == 1
jsonpath "$.artifacts[0].os" == null
jsonpath "$.checksums.name" == "checksums.txt"

# deprecated as a message.
GET {{url}}/legacy-tool/releases/latest
HTTP 200
[Asserts]
jsonpath "$.deprecated" == "replaced by gitlab"

# Not found: unknown version, unknown product, invalid name.
GET {{url}}/gitlab/releases/v9.9.9
HTTP 404
[Asserts]
jsonpath "$.error" == "not found"

GET {{url}}/missing/releases/latest
HTTP 404

GET {{url}}/gitlab/releases/..%2F..
HTTP 404
