# GET /{product}/artifacts
# artifacts.json of the latest version, as rewritten at publish: each path is the file's download URL.

GET {{url}}/gitlab/artifacts
HTTP 200
[Captures]
metadata_url: jsonpath "$[0].path"
[Asserts]
header "Content-Type" == "application/json"
# The build output entry (Binary without extra.Format) names no uploaded file: publish dropped it.
jsonpath "$" count == 4
jsonpath "$[0].name" == "metadata.json"
jsonpath "$[0].type" == "Metadata"
jsonpath "$[0].path" == "{{url}}/gitlab/artifacts/v0.1.0/files/metadata.json"
jsonpath "$[1].name" == "gitlab_0.1.0_linux_amd64.tar.gz"
jsonpath "$[1].type" == "Archive"
jsonpath "$[1].goos" == "linux"
jsonpath "$[1].path" == "{{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz"
jsonpath "$[2].extra.Checksum" == "sha256:8ce6a6ede1b221e225d57e882740143ad598fa99c5493296086466b9f119f08f"
jsonpath "$[3].type" == "Checksum"

# Each path downloads the file.
GET {{metadata_url}}
HTTP 200
[Asserts]
sha256 == hex,471377929f0abc1c161d5738a78359da735ae34c20e9b50d8c027bb012a5f2de;

# A latest version published without artifacts.json (folder mode) has none.
GET {{url}}/billing-db-migrations/artifacts
HTTP 404
[Asserts]
jsonpath "$.error" == "not found"

GET {{url}}/missing/artifacts
HTTP 404
