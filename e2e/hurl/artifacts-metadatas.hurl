# GET /artifacts/metadatas/{product}?version=
# metadata.json written by goreleaser for a version; without version, or "latest", the latest version.

GET {{url}}/artifacts/metadatas/gitlab
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.project_name" == "gitlab"
jsonpath "$.tag" == "v0.1.0"
jsonpath "$.version" == "0.1.0"
jsonpath "$.commit" == "9c9d7664ceef"
jsonpath "$.runtime.goos" == "linux"

GET {{url}}/artifacts/metadatas/gitlab
[Query]
version: v0.1.0
HTTP 200
[Asserts]
jsonpath "$.tag" == "v0.1.0"

GET {{url}}/artifacts/metadatas/gitlab
[Query]
version: latest
HTTP 200
[Asserts]
jsonpath "$.tag" == "v0.1.0"

GET {{url}}/artifacts/metadatas/gitlab
[Query]
version:
HTTP 200
[Asserts]
jsonpath "$.tag" == "v0.1.0"

# A version published without metadata.json.
GET {{url}}/artifacts/metadatas/gitlab
[Query]
version: v0.2.0-rc.1
HTTP 404
[Asserts]
jsonpath "$.error" == "not found"

GET {{url}}/artifacts/metadatas/billing-db-migrations
HTTP 404

GET {{url}}/artifacts/metadatas/gitlab
[Query]
version: v9.9.9
HTTP 404

GET {{url}}/artifacts/metadatas/missing
HTTP 404
