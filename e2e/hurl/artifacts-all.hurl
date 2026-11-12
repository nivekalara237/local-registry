# GET /artifacts/all
# Raw catalogue: for each directory of bin.local/assets holding a known file, the content of
# artifacts.json, metadata.json and *_checksums.txt as strings. Sorted by directory path.

GET {{url}}/artifacts/all
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
# Only gitlab has an artifacts.json: its latest copy, then its version directory v0.1.0.
jsonpath "$" count == 2
jsonpath "$[0].provider" == "gitlab"
jsonpath "$[0].artifacts_json" contains "gitlab_0.1.0_linux_amd64.tar.gz"
jsonpath "$[0].metadata_json" == ""
jsonpath "$[0].checksums_text" == ""
# provider is the name of the directory: the version, for a version directory.
jsonpath "$[1].provider" == "v0.1.0"
jsonpath "$[1].artifacts_json" contains "{{url}}/gitlab/artifacts/v0.1.0/files/"
jsonpath "$[1].metadata_json" contains "\"tag\":\"v0.1.0\""
jsonpath "$[1].checksums_text" contains "  gitlab_0.1.0_linux_amd64.tar.gz"
